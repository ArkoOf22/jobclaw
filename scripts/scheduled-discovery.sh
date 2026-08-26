#!/usr/bin/env bash
#
# Scheduled discovery: fetch new jobs, then score them.
#
# Run by jobclaw-discover.service on a timer. Deliberately stops at scoring:
# approval is a human gate, so nothing downstream of it happens unattended.
#
# Safe to run repeatedly. Discovery upserts on (source, external_id), and scoring
# only advances a job to SHORTLISTED, which cannot clobber an approval.
#
set -euo pipefail

readonly PROJECT_DIR="${JOBCLAW_DIR:-/home/openclaw/jobclaw}"
readonly GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
readonly BINARY="${PROJECT_DIR}/bin/jobclaw"

cd "${PROJECT_DIR}"

# Load configuration. The file holds secrets, so it is sourced rather than
# logged, and `set -a` exports everything it defines.
if [[ -f .env ]]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

log() {
    printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

# Rebuild when the binary is missing or older than any tracked source file, so a
# deploy does not silently keep running stale code.
needs_build=0

if [[ ! -x "${BINARY}" ]]; then
    needs_build=1
elif [[ -n "$(find cmd internal -name '*.go' -newer "${BINARY}" -print -quit 2>/dev/null)" ]]; then
    needs_build=1
fi

if [[ "${needs_build}" -eq 1 ]]; then
    log "building ${BINARY}"
    mkdir -p "${PROJECT_DIR}/bin"
    "${GO_BIN}" build -o "${BINARY}" ./cmd/jobclaw
fi

log "discovery starting"

# Discovery must not abort the run: one source failing is expected and is already
# isolated internally, and scoring should still process whatever landed.
if ! "${BINARY}" discover; then
    log "discovery reported a failure; continuing to scoring"
fi

log "scoring starting"

"${BINARY}" score

log "run complete"

# Leave a human-readable summary in the journal so `journalctl -u
# jobclaw-discover` is useful without querying the database.
"${BINARY}" status
