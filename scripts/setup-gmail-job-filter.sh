#!/usr/bin/env bash
#
# Create the Gmail filter that labels job mail, so the scheduled checker can
# read only job mail and never touch the rest of the inbox.
#
# Idempotent-ish: it refuses to create a second filter carrying the same label,
# so re-running is safe. Pass --dry-run to see what would happen.
#
# Why a filter at all, when the checker could just run this query directly:
# labelling narrows what the checker ever reads to one label. A query that has
# to scan the whole inbox is the thing we are trying to get away from.
#
set -euo pipefail

readonly LABEL="JobClaw/Mail"
readonly PROJECT_DIR="${JOBCLAW_DIR:-/home/openclaw/jobclaw}"

DRY_RUN=""
if [[ "${1:-}" == "--dry-run" || "${1:-}" == "-n" ]]; then
    DRY_RUN="--dry-run"
fi

cd "${PROJECT_DIR}"

# gog needs GOG_KEYRING_PASSWORD, which lives in .env alongside the sheet config.
set -a
# shellcheck disable=SC1091
[[ -f .env ]] && . ./.env
set +a

readonly GOG="${JOBCLAW_GOG_BIN:-gog}"

# Sender domains. Deliberately inclusive.
#
# Because the checker batches -- one model call per run regardless of how many
# messages matched -- a false positive costs essentially nothing, while a false
# negative means a missed interview invitation. So this errs toward catching too
# much.
#
# The first group was measured from a 14-day sample of the real inbox; the rest
# are boards and ATS platforms that plausibly appear later. Listing a domain that
# never sends is free.
readonly SENDERS=(
    # measured present in this account
    linkedin.com
    naukri.com
    wellfound.com
    myworkday.com
    icims.com
    uplers.network
    autoapplymax.com
    uhg.com
    # major ATS platforms
    greenhouse.io
    lever.co
    ashbyhq.com
    smartrecruiters.com
    jobvite.com
    taleo.net
    workable.com
    successfactors.com
    darwinbox.com
    keka.com
    freshteam.com
    # India-focused boards
    hirist.com
    instahyre.com
    cutshort.io
    iimjobs.com
    foundit.in
    shine.com
    # global boards
    indeed.com
    glassdoor.com
    monster.com
    dice.com
    ziprecruiter.com
)

# Subject phrases, to catch recruiters mailing from ordinary corporate domains
# that no sender list can predict. Kept to phrases that are specific to an
# application in flight, so marketing mail shouting "we're hiring" mostly misses.
readonly SUBJECTS=(
    "your application"
    "application received"
    "application status"
    "job application"
    "interview"
    "shortlisted"
    "candidature"
    "offer letter"
    "coding assessment"
    "online assessment"
    "hiring process"
    "recruitment process"
)

join_or() {
    local IFS="|"
    local joined="$*"
    printf '%s' "${joined// / OR }"
}

senders_clause="from:($(
    IFS=$'\n'
    printf '%s OR ' "${SENDERS[@]}" | sed 's/ OR $//'
))"

subjects_clause="subject:($(
    for s in "${SUBJECTS[@]}"; do printf '"%s" OR ' "$s"; done | sed 's/ OR $//'
))"

QUERY="${senders_clause} OR ${subjects_clause}"

echo "Label : ${LABEL}"
echo "Query : ${QUERY}"
echo

# Refuse to add a duplicate. Gmail will happily hold two identical filters and
# then apply the label twice, which is harmless but confusing to audit later.
existing="$("${GOG}" gmail filters list --json 2>/dev/null || echo '{}')"

if printf '%s' "${existing}" | grep -q "${LABEL}"; then
    echo "A filter already references ${LABEL}. Nothing to do."
    echo "Inspect with: ${GOG} gmail filters list --json"
    exit 0
fi

if printf '%s' "${existing}" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(1)
sys.exit(0 if (d.get("filters") or []) else 1)
'; then
    echo "note: this account already has other Gmail filters; leaving them alone."
fi

# shellcheck disable=SC2086
"${GOG}" gmail filters create \
    --query "${QUERY}" \
    --add-label "${LABEL}" \
    --json \
    ${DRY_RUN}

echo
if [[ -n "${DRY_RUN}" ]]; then
    echo "Dry run only. Re-run without --dry-run to create it."
else
    echo "Created. Note Gmail applies filters to NEW mail only, so existing"
    echo "messages stay unlabelled. The checker looks at the label, so it will"
    echo "start seeing job mail from the next message onward."
fi
