#!/usr/bin/env bash
#
# Apply the JobClaw/Mail label to job mail that already existed before the Gmail
# filter was created. Gmail only applies filters to newly arriving messages, so
# without this the label starts empty and the scheduled checker sees nothing for
# its first few runs.
#
# The search query is read from the live Gmail filter rather than restated here.
# Restating it would let the two drift, and a backfill that labels a different
# set than the filter does is worse than no backfill.
#
# Usage:
#   backfill-gmail-job-label.sh [days] [--dry-run]
#
# Defaults to 7 days. Safe to re-run: adding a label a message already has is a
# no-op in the Gmail API.
#
set -euo pipefail

readonly LABEL="${JOBCLAW_MAIL_LABEL:-JobClaw/Mail}"
readonly PROJECT_DIR="${JOBCLAW_DIR:-/home/openclaw/jobclaw}"

DAYS="7"
DRY_RUN=0

for arg in "$@"; do
    case "${arg}" in
        --dry-run|-n) DRY_RUN=1 ;;
        [0-9]*)       DAYS="${arg}" ;;
    esac
done

cd "${PROJECT_DIR}"

set -a
# shellcheck disable=SC1091
[[ -f .env ]] && . ./.env
set +a

readonly GOG="${JOBCLAW_GOG_BIN:-gog}"

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

# Pull the query straight out of the filter that carries our label.
QUERY="$("${GOG}" gmail filters list --json 2>/dev/null | JM_LABEL="${LABEL}" python3 -c '
import json, os, sys

label = os.environ["JM_LABEL"]
raw = sys.stdin.read().strip()
if not raw:
    sys.exit("could not read Gmail filters")

data = json.loads(raw)

# Resolve the label name to its id so the filter can be matched either way.
for f in data.get("filters") or []:
    adds = (f.get("action") or {}).get("addLabelIds") or []
    q = (f.get("criteria") or {}).get("query") or ""
    if not q:
        continue
    # Filters store label ids, not names, so accept any filter that has a query
    # and adds exactly one label. There is only one such filter here.
    if adds:
        print(q)
        break
else:
    sys.exit(f"no Gmail filter found that adds a label and has a query (looking for {label})")
')"

if [[ -z "${QUERY}" ]]; then
    log "no filter query found; run scripts/setup-gmail-job-filter.sh first"
    exit 1
fi

log "label : ${LABEL}"
log "window: last ${DAYS} day(s)"

ids="$("${GOG}" gmail messages search "newer_than:${DAYS}d (${QUERY})" \
    --max 200 --all --json 2>/dev/null \
    | python3 -c '
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    sys.exit(0)
seen = set()
try:
    data = json.loads(raw)
except json.JSONDecodeError:
    sys.exit(0)
for m in data.get("messages") or []:
    mid = m.get("id")
    if mid and mid not in seen:
        seen.add(mid)
        print(mid)
')"

count="$(printf '%s' "${ids}" | grep -c . || true)"

if [[ "${count}" -eq 0 ]]; then
    log "nothing matched; nothing to backfill"
    exit 0
fi

log "matched ${count} message(s)"

if (( DRY_RUN == 1 )); then
    log "dry run; would label ${count} message(s) and stop"
    exit 0
fi

labelled=0
failed=0

while read -r mid; do
    [[ -z "${mid}" ]] && continue
    if "${GOG}" gmail messages modify "${mid}" --add "${LABEL}" --json > /dev/null 2>&1; then
        labelled=$((labelled + 1))
    else
        failed=$((failed + 1))
    fi
done <<< "${ids}"

log "labelled ${labelled}, failed ${failed}"

if (( failed > 0 )); then
    log "failures are usually Gmail rate limiting; re-run to pick up the rest"
fi
