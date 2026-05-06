#!/usr/bin/env bash
# PreToolUse hook: block any `git push --force` / `-f` / `--force-with-lease`
# variant. Branch protection on the GitHub side is the durable defense; this
# is a local guardrail so a stray Claude command can't rewrite published
# history on `main` (or anywhere else).
#
# Hook input contract: a JSON object on stdin. For Bash tool calls,
# `.tool_input.command` is the literal shell string about to run.
# Exit codes: 0 = allow, 2 = block (stderr → user), other = silent error.

set -euo pipefail

input="$(cat)"

# Pull out the command. Use python rather than jq so this works on a fresh
# OCI VM with only the standard toolchain.
cmd="$(printf '%s' "$input" | python3 -c '
import json, sys
try:
    obj = json.load(sys.stdin)
except Exception:
    sys.exit(0)
ti = obj.get("tool_input") or {}
print(ti.get("command", ""), end="")
')"

# Only inspect git push invocations. Match `git push` at the start of a
# statement — i.e. either the very beginning of the command line, or right
# after a shell separator (`;`, `&&`, `||`, `|`). This avoids false positives
# on innocent strings like `echo git push -f`.
if ! printf '%s' "$cmd" | grep -qE '(^|[;&|])[[:space:]]*git[[:space:]]+push([[:space:];&|]|$)'; then
  exit 0
fi

# Reject any force variant, in any arg position.
if printf '%s' "$cmd" | grep -qE '([[:space:]])(--force(-with-lease)?(=[^[:space:]]*)?|-f|-[A-Za-z]*f[A-Za-z]*)([[:space:];&|]|$)'; then
  echo "BLOCKED: force-push variants are denied by .claude/hooks/block-force-push.sh." >&2
  echo "Command: $cmd" >&2
  echo "If you genuinely need this, run the command outside Claude or temporarily disable the hook." >&2
  exit 2
fi

exit 0
