#!/usr/bin/env bash
#
# Scheduled job-mail check. Replaces the Gmail push hook, which woke the agent
# once per inbox email -- roughly 39 model calls a day, most of them for bank
# statements and shopping newsletters.
#
# Two properties this is built around:
#
#   1. It reads ONLY the JobClaw/Mail label. Never the inbox, never anything
#      else. The Gmail filter decides what earns that label.
#   2. A run with no new job mail makes ZERO model calls and costs nothing.
#      Only a run that found something spends tokens, and then exactly once for
#      the whole batch rather than once per message.
#
# Run by jobclaw-mail.timer. Safe to run by hand.
#
set -euo pipefail

readonly PROJECT_DIR="${JOBCLAW_DIR:-/home/openclaw/jobclaw}"
readonly LABEL="${JOBCLAW_MAIL_LABEL:-JobClaw/Mail}"
readonly STATE_FILE="${PROJECT_DIR}/data/job-mail-last-seen"

# Never look further back than this, so a long outage cannot produce one
# enormous digest. Anything older is simply considered already seen.
readonly MAX_LOOKBACK_SECONDS=$((7 * 24 * 3600))

cd "${PROJECT_DIR}"

set -a
# shellcheck disable=SC1091
[[ -f .env ]] && . ./.env
set +a

readonly GOG="${JOBCLAW_GOG_BIN:-gog}"

log() {
    printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" || "${1:-}" == "-n" ]]; then
    DRY_RUN=1
fi

now="$(date -u +%s)"
floor=$((now - MAX_LOOKBACK_SECONDS))

if [[ -f "${STATE_FILE}" ]]; then
    last_seen="$(tr -cd '0-9' < "${STATE_FILE}")"
    last_seen="${last_seen:-0}"
else
    # First ever run: look back a single day rather than replaying history.
    last_seen=$((now - 86400))
fi

if (( last_seen < floor )); then
    log "last seen ${last_seen} is older than the ${MAX_LOOKBACK_SECONDS}s floor; clamping"
    last_seen="${floor}"
fi

# Gmail accepts an epoch for after:. Scoped to the label, so this query can
# never surface a message the filter did not mark as job mail.
readonly QUERY="label:${LABEL} after:${last_seen}"

log "querying: ${QUERY}"

payload="$("${GOG}" gmail messages search "${QUERY}" \
    --max 50 --json 2>/dev/null || true)"

if [[ -z "${payload}" ]]; then
    log "gmail query returned nothing usable (auth or rate limit?); leaving state untouched"
    exit 0
fi

# Compact digest: sender domain and subject only. Bodies are deliberately not
# fetched -- they are what made the old hook expensive, and a subject line is
# enough to decide whether something needs attention.
digest="$(printf '%s' "${payload}" | python3 -c '
import json, re, sys

raw = sys.stdin.read().strip()
if not raw:
    sys.exit(0)

try:
    data = json.loads(raw)
except json.JSONDecodeError:
    sys.exit(0)

msgs = data.get("messages") or []
lines = []

for m in msgs:
    frm = m.get("from", "")
    dom = re.search(r"@([A-Za-z0-9.\-]+)", frm)
    dom = dom.group(1).lower().rstrip(">.") if dom else "unknown"
    subj = (m.get("subject") or "(no subject)").strip()[:120]
    date = (m.get("date") or "")[:16]
    lines.append(f"- [{date}] {dom}: {subj}")

print("\n".join(lines))
')"

count="$(printf '%s' "${digest}" | grep -c '^- ' || true)"

if [[ "${count}" -eq 0 ]]; then
    log "no new job mail; 0 model calls, nothing spent"
    # Advance the watermark anyway so the window does not creep wider forever.
    if (( DRY_RUN == 0 )); then
        mkdir -p "$(dirname "${STATE_FILE}")"
        printf '%s\n' "${now}" > "${STATE_FILE}"
    fi
    exit 0
fi

log "found ${count} new job mail item(s); making ONE batched agent call"

read -r -d '' PROMPT <<PROMPT_EOF || true
You are reviewing new job-related email for Arkodeep. ${count} message(s)
arrived since the last check:

${digest}

Do this:

1. Separate genuine application activity (an employer replying, an interview
   invitation, an assessment request, a status change, a rejection) from routine
   job-board alerts and marketing. Say plainly which is which.
2. For anything that looks like real application activity, check whether the
   company already exists in JobClaw by running: jobclaw-agent status --json
   If it matches a job or application there, say which one, with its id.
3. If something needs Arkodeep to act, say exactly what and give the command.
4. If it is all routine alerts, say so in one line. Do not pad it.

Only these subject lines and sender domains are available; email bodies were
deliberately not fetched. Do not guess at content you cannot see, and do not
invent job ids.
PROMPT_EOF

if (( DRY_RUN == 1 )); then
    log "dry run; would send this prompt:"
    printf '%s\n' "${PROMPT}"
    log "dry run; state file not advanced"
    exit 0
fi

# A fresh session key per run keeps conversation history near zero, which is
# 54% of a typical request. The digest above is the only context needed, and the
# state file -- not the model -- is what remembers what was already reported.
session_key="jobmail-$(date -u +%Y%m%d-%H%M)"

# Telegram delivery needs an explicit chat id. Derive it from the configured
# owner rather than hardcoding, so this keeps working if the account changes.
chat_id="${JOBCLAW_MAIL_TELEGRAM_CHAT:-}"

if [[ -z "${chat_id}" ]]; then
    chat_id="$(openclaw config get commands.ownerAllowFrom 2>/dev/null \
        | python3 -c '
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    sys.exit(0)
try:
    entries = json.loads(raw)
except json.JSONDecodeError:
    sys.exit(0)
if isinstance(entries, str):
    entries = [entries]
for e in entries or []:
    if isinstance(e, str) and e.startswith("telegram:"):
        print(e.split(":", 1)[1])
        break
')"
fi

if [[ -z "${chat_id}" ]]; then
    log "no telegram chat id found (set JOBCLAW_MAIL_TELEGRAM_CHAT or commands.ownerAllowFrom)"
    log "printing the digest here instead so the run is not wasted:"
    printf '%s\n' "${digest}"
    exit 1
fi

log "delivering to telegram:${chat_id}"

if openclaw agent \
    --session-key "${session_key}" \
    --message "${PROMPT}" \
    --deliver \
    --channel telegram \
    --reply-to "${chat_id}" \
    --thinking low \
    --timeout 300 2>&1 | sed 's/^/    /'
then
    log "digest delivered; advancing watermark"
    mkdir -p "$(dirname "${STATE_FILE}")"
    printf '%s\n' "${now}" > "${STATE_FILE}"
else
    log "agent call failed; leaving watermark so the next run retries this batch"
    exit 1
fi
