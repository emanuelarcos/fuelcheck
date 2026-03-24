#!/usr/bin/env python3

import json
import os
import sys
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path


USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None

RESET = "\033[0m" if USE_COLOR else ""
BOLD = "\033[1m" if USE_COLOR else ""
DIM = "\033[2m" if USE_COLOR else ""
CYAN = "\033[36m" if USE_COLOR else ""
BLUE = "\033[94m" if USE_COLOR else ""
GREEN = "\033[92m" if USE_COLOR else ""
YELLOW = "\033[93m" if USE_COLOR else ""
MAGENTA = "\033[95m" if USE_COLOR else ""

OAUTH_USAGE_URL = "https://api.anthropic.com/api/oauth/usage"
ORGS_URL = "https://claude.ai/api/organizations"
CLAUDE_CREDENTIALS_PATH = Path.home() / ".claude" / ".credentials.json"


def style(text, *codes):
    if not USE_COLOR:
        return text
    return "".join(codes) + text + RESET


def request_json(url, headers):
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def remaining(window):
    value = window.get("utilization")
    if value is None:
        return "-"
    try:
        if float(value) <= 1:
            used_percent = float(value) * 100
        else:
            used_percent = float(value)
        left = max(0, round(100 - used_percent))
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


def fmt_spanish_time(value, long_date=False):
    if not value:
        return "-"
    if isinstance(value, (int, float)):
        dt = datetime.fromtimestamp(value)
    else:
        text = str(value).replace("Z", "+00:00")
        dt = datetime.fromisoformat(text)
        dt = dt.astimezone()
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


def resolve_web_session_key():
    direct = (
        os.environ.get("CLAUDE_AI_SESSION_KEY", "").strip()
        or os.environ.get("CLAUDE_WEB_SESSION_KEY", "").strip()
    )
    if direct.startswith("sk-ant-"):
        return direct
    cookie_header = os.environ.get("CLAUDE_WEB_COOKIE", "").strip()
    if not cookie_header:
        return None
    cookie_header = (
        cookie_header.removeprefix("Cookie:").removeprefix("cookie:").strip()
    )
    for part in cookie_header.split(";"):
        key, _, value = part.strip().partition("=")
        if key == "sessionKey" and value.strip().startswith("sk-ant-"):
            return value.strip()
    return None


def fetch_web_usage(session_key):
    headers = {
        "Cookie": f"sessionKey={session_key}",
        "Accept": "application/json",
        "User-Agent": "claude-usage",
    }
    orgs = request_json(ORGS_URL, headers)
    if not orgs:
        raise RuntimeError("No se encontraron organizaciones de Claude")
    org = orgs[0]
    org_id = org.get("uuid")
    if not org_id:
        raise RuntimeError("No se pudo obtener el org id de Claude")
    usage = request_json(f"{ORGS_URL}/{org_id}/usage", headers)
    return {"usage": usage, "org": org, "source": "web"}


def fetch_oauth_usage(token):
    headers = {
        "Authorization": f"Bearer {token}",
        "User-Agent": "claude-usage",
        "Accept": "application/json",
        "anthropic-version": "2023-06-01",
        "anthropic-beta": "oauth-2025-04-20",
    }
    usage = request_json(OAUTH_USAGE_URL, headers)
    return {"usage": usage, "org": None, "source": "oauth"}


def load_local_claude_credentials():
    if not CLAUDE_CREDENTIALS_PATH.exists():
        return {}
    try:
        return json.loads(CLAUDE_CREDENTIALS_PATH.read_text())
    except Exception:
        return {}


def load_local_claude_oauth_token():
    creds = load_local_claude_credentials()
    return creds.get("claudeAiOauth", {}).get("accessToken")


def load_local_claude_account():
    creds = load_local_claude_credentials()
    oauth = creds.get("claudeAiOauth", {})
    return {
        "subscriptionType": oauth.get("subscriptionType"),
        "rateLimitTier": oauth.get("rateLimitTier"),
    }


def fetch_usage():
    token = (
        os.environ.get("CLAUDE_CODE_OAUTH_TOKEN", "").strip()
        or os.environ.get("ANTHROPIC_AUTH_TOKEN", "").strip()
        or (load_local_claude_oauth_token() or "")
    )
    if token:
        try:
            payload = fetch_oauth_usage(token)
            payload["account"] = load_local_claude_account()
            if not os.environ.get("CLAUDE_CODE_OAUTH_TOKEN") and not os.environ.get(
                "ANTHROPIC_AUTH_TOKEN"
            ):
                payload["source"] = "local oauth"
            return payload
        except urllib.error.HTTPError as e:
            if e.code != 403:
                raise
    session_key = resolve_web_session_key()
    if session_key:
        return fetch_web_usage(session_key)
    raise RuntimeError(
        "Falta auth de Claude. Exporta CLAUDE_AI_SESSION_KEY (o CLAUDE_WEB_SESSION_KEY / CLAUDE_WEB_COOKIE). "
        "Si tenes un token OAuth, tambien sirve CLAUDE_CODE_OAUTH_TOKEN."
    )


def print_summary(payload):
    data = payload["usage"]
    org = payload.get("org") or {}
    account = payload.get("account") or {}
    plan = (
        org.get("name")
        or org.get("display_name")
        or org.get("uuid")
        or account.get("subscriptionType")
        or "Claude"
    )
    if isinstance(plan, str):
        plan = plan[:1].upper() + plan[1:]

    print(f"{style('Plan:', BOLD, CYAN)} {style(plan, BOLD, BLUE)}")
    if account.get("rateLimitTier"):
        print(
            f"{style('Tier:', BOLD, CYAN)} {style(str(account['rateLimitTier']), BOLD)}"
        )
    print(f"{style('Fuente:', BOLD, CYAN)} {payload.get('source', '-')}")
    print("")

    if data.get("five_hour"):
        left = remaining(data["five_hour"])
        print(style("Límite de uso de 5 horas:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )
        print(
            f"{style('Se restablecerá:', DIM)} {fmt_spanish_time(data['five_hour'].get('resets_at'))}"
        )
        print("")

    if data.get("seven_day"):
        left = remaining(data["seven_day"])
        print(style("Límite de uso semanal:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )
        print(
            f"{style('Se restablecerá:', DIM)} {fmt_spanish_time(data['seven_day'].get('resets_at'), long_date=True)}"
        )
        print("")

    if data.get("seven_day_sonnet"):
        left = remaining(data["seven_day_sonnet"])
        print(style("Límite semanal Sonnet:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )
        print("")

    if data.get("seven_day_opus"):
        left = remaining(data["seven_day_opus"])
        print(style("Límite semanal Opus:", BOLD, CYAN))
        print("")
        print(
            f"{progress_bar(left)} {style(f'{left}% restante', BOLD, accent_for_remaining(left))}"
        )


def main():
    raw = "--json" in sys.argv[1:]
    payload = fetch_usage()
    if raw:
        print(json.dumps(payload, indent=2))
    else:
        print_summary(payload)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        print(f"claude-usage failed: HTTP {e.code} {body}".strip(), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"claude-usage failed: {e}", file=sys.stderr)
        sys.exit(1)
