#!/usr/bin/env python3

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path


USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None
RESET = "\033[0m" if USE_COLOR else ""
BOLD = "\033[1m" if USE_COLOR else ""
DIM = "\033[2m" if USE_COLOR else ""
CYAN = "\033[36m" if USE_COLOR else ""
BLUE = "\033[94m" if USE_COLOR else ""

BASE_DIR = Path(__file__).resolve().parent


def style(text, *codes):
    if not USE_COLOR:
        return text
    return "".join(codes) + text + RESET


def resolve_runner(name):
    command = shutil.which(name)
    if command:
        return [command]
    local = BASE_DIR / f"{name}.py"
    if local.exists():
        return [sys.executable, str(local)]
    return None


def run_provider(name):
    runner = resolve_runner(name)
    if not runner:
        return {"ok": False, "error": f"No encontré {name}", "provider": name}
    env = dict(os.environ)
    env.setdefault("NO_COLOR", "1")
    result = subprocess.run(
        runner + ["--json"],
        capture_output=True,
        text=True,
        env=env,
    )
    if result.returncode != 0:
        return {
            "ok": False,
            "error": (result.stderr or result.stdout).strip() or "falló sin salida",
            "provider": name,
        }
    try:
        return {"ok": True, "data": json.loads(result.stdout), "provider": name}
    except json.JSONDecodeError as e:
        return {"ok": False, "error": f"JSON inválido: {e}", "provider": name}


def codex_lines(data):
    plan = data.get("plan_type", "-")
    primary = data.get("rate_limit", {}).get("primary_window", {})
    secondary = data.get("rate_limit", {}).get("secondary_window", {})
    lines = [f"Plan: {str(plan).capitalize()}"]
    if primary:
        left = max(0, round(100 - float(primary.get("used_percent", 0))))
        lines.append(f"5h: {left}% restante")
    if secondary:
        left = max(0, round(100 - float(secondary.get("used_percent", 0))))
        lines.append(f"Semana: {left}% restante")
    return lines


def claude_lines(data):
    usage = data.get("usage", {})
    account = data.get("account", {})
    plan = account.get("subscriptionType") or "Claude"
    lines = [f"Plan: {str(plan).capitalize()}"]
    if account.get("rateLimitTier"):
        lines.append(f"Tier: {account['rateLimitTier']}")
    if usage.get("five_hour"):
        util = float(usage["five_hour"].get("utilization") or 0)
        left = max(0, round(100 - util))
        lines.append(f"5h: {left}% restante")
    if usage.get("seven_day"):
        util = float(usage["seven_day"].get("utilization") or 0)
        left = max(0, round(100 - util))
        lines.append(f"Semana: {left}% restante")
    return lines


def print_block(title, lines):
    print(style(title, BOLD, CYAN))
    for line in lines:
        print(f"  {line}")


def main():
    results = {
        "codex": run_provider("codex-usage"),
        "claude": run_provider("claude-usage"),
    }

    if "--json" in sys.argv[1:]:
        payload = {}
        for key, result in results.items():
            payload[key] = (
                result["data"] if result["ok"] else {"error": result["error"]}
            )
        print(json.dumps(payload, indent=2))
        return

    printed = False
    if results["codex"]["ok"]:
        print_block("Codex", codex_lines(results["codex"]["data"]))
        printed = True
    if results["claude"]["ok"]:
        if printed:
            print("")
        print_block("Claude", claude_lines(results["claude"]["data"]))
        printed = True

    if not printed:
        errors = [f"{k}: {v['error']}" for k, v in results.items() if not v["ok"]]
        print("llm-usage failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        sys.exit(1)

    failures = [v for v in results.values() if not v["ok"]]
    if failures:
        print("")
        print(style("No disponibles", BOLD, BLUE))
        for failure in failures:
            print(f"  {failure['provider']}: {failure['error']}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
