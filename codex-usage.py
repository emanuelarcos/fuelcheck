#!/usr/bin/env python3

import base64
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime
from pathlib import Path


AUTH_PATH = Path.home() / ".codex" / "auth.json"
TOKEN_URL = "https://auth.openai.com/oauth/token"
USAGE_URL = "https://chatgpt.com/backend-api/wham/usage"
CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None

RESET = "\033[0m" if USE_COLOR else ""
BOLD = "\033[1m" if USE_COLOR else ""
DIM = "\033[2m" if USE_COLOR else ""
CYAN = "\033[36m" if USE_COLOR else ""
BLUE = "\033[94m" if USE_COLOR else ""
GREEN = "\033[92m" if USE_COLOR else ""
YELLOW = "\033[93m" if USE_COLOR else ""
MAGENTA = "\033[95m" if USE_COLOR else ""


def load_auth():
    with AUTH_PATH.open() as f:
        return json.load(f)


def save_auth(auth):
    AUTH_PATH.write_text(json.dumps(auth, indent=2) + "\n")


def request_json(url, headers=None, data=None):
    req = urllib.request.Request(url, headers=headers or {}, data=data)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def refresh_tokens(refresh_token):
    data = urllib.parse.urlencode(
        {
            "grant_type": "refresh_token",
            "refresh_token": refresh_token,
            "client_id": CLIENT_ID,
        }
    ).encode()
    return request_json(
        TOKEN_URL,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        data=data,
    )


def usage_headers(auth):
    headers = {
        "Authorization": f"Bearer {auth['tokens']['access_token']}",
        "Accept": "application/json",
        "User-Agent": "CodexBar",
    }
    account_id = auth.get("tokens", {}).get("account_id")
    if account_id:
        headers["ChatGPT-Account-Id"] = account_id
    return headers


def fetch_usage(auth):
    return request_json(USAGE_URL, headers=usage_headers(auth))


def fmt_spanish_time(ts, long_date=False):
    if not ts:
        return "-"
    dt = datetime.fromtimestamp(ts)
    months = {
        1: "ene",
        2: "feb",
        3: "mar",
        4: "abr",
        5: "may",
        6: "jun",
        7: "jul",
        8: "ago",
        9: "sep",
        10: "oct",
        11: "nov",
        12: "dic",
    }
    hour = dt.hour % 12 or 12
    ampm = "a.m." if dt.hour < 12 else "p.m."
    minute = f"{dt.minute:02d}"
    if long_date:
        return f"{dt.day} {months[dt.month]} {dt.year} {hour}:{minute} {ampm}"
    return f"{hour}:{minute} {ampm}"


def style(text, *codes):
    if not USE_COLOR:
        return text
    return "".join(codes) + text + RESET


def remaining(window):
    value = window.get("used_percent")
    if value is None:
        return "-"
    try:
        left = max(0, round(100 - float(value)))
        return str(left)
    except Exception:
        return "-"


def accent_for_remaining(value):
    try:
        pct_left = float(value)
    except Exception:
        return CYAN
    if pct_left >= 70:
        return GREEN
    if pct_left >= 35:
        return YELLOW
    return MAGENTA


def progress_bar(value, width=22):
    try:
        pct_left = max(0, min(100, float(value)))
    except Exception:
        return ""
    filled = round((pct_left / 100) * width)
    empty = width - filled
    color = accent_for_remaining(pct_left)
    return style("█" * filled, color) + style("░" * empty, DIM)


def extract_email(auth):
    email = auth.get("user", {}).get("email") or auth.get("email")
    if email:
        return email
    try:
        id_token = auth.get("tokens", {}).get("id_token", "")
        payload = id_token.split(".")[1]
        payload += "=" * (-len(payload) % 4)
        return json.loads(base64.urlsafe_b64decode(payload)).get("email")
    except Exception:
        return None


def print_summary(data, auth):
    primary = data.get("rate_limit", {}).get("primary_window", {})
    secondary = data.get("rate_limit", {}).get("secondary_window", {})
    plan = data.get("plan_type", "-")
    email = extract_email(auth)
    if isinstance(plan, str):
        plan = plan[:1].upper() + plan[1:]

    print(f"{style('Plan:', BOLD, CYAN)} {style(str(plan), BOLD, BLUE)}")
    if email:
        print(f"{style('Email:', BOLD, CYAN)} {style(email, BOLD)}")
    print("")

    if primary:
        left = remaining(primary)
        print(style("Límite de uso de 5 horas:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )
        print(
            f"{style('Se restablecerá:', DIM)} {fmt_spanish_time(primary.get('reset_at'))}"
        )
        print("")

    if secondary:
        left = remaining(secondary)
        print(style("Límite de uso semanal:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )
        print(
            f"{style('Se restablecerá:', DIM)} {fmt_spanish_time(secondary.get('reset_at'), long_date=True)}"
        )


def main():
    if not AUTH_PATH.exists():
        print("Missing ~/.codex/auth.json. Run: codex login", file=sys.stderr)
        sys.exit(1)

    raw = "--json" in sys.argv[1:]
    auth = load_auth()

    try:
        data = fetch_usage(auth)
    except urllib.error.HTTPError as e:
        if e.code not in (401, 403):
            body = e.read().decode("utf-8", errors="replace")
            print(
                f"Usage request failed: HTTP {e.code} {body}".strip(), file=sys.stderr
            )
            sys.exit(1)

        refresh_token = auth.get("tokens", {}).get("refresh_token")
        if not refresh_token:
            print("Session expired. Run: codex login", file=sys.stderr)
            sys.exit(1)

        try:
            refreshed = refresh_tokens(refresh_token)
        except urllib.error.HTTPError:
            print(
                "Session expired and refresh failed. Run: codex login", file=sys.stderr
            )
            sys.exit(1)

        auth["tokens"]["access_token"] = refreshed["access_token"]
        auth["tokens"]["refresh_token"] = refreshed.get("refresh_token", refresh_token)
        save_auth(auth)

        try:
            data = fetch_usage(auth)
        except urllib.error.HTTPError:
            print("Usage fetch still unauthorized. Run: codex login", file=sys.stderr)
            sys.exit(1)

    if raw:
        print(json.dumps(data, indent=2))
    else:
        print_summary(data, auth)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as e:
        print(f"codex-usage failed: {e}", file=sys.stderr)
        sys.exit(1)
