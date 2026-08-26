#!/usr/bin/env bash
#
# Set any secret in the .env on the EC2 host, without it appearing anywhere it
# should not.
#
# Generalises set-openrouter-key.sh, which remains for that specific key. Same
# four leak paths are closed:
#   1. Terminal echo    -- `read -rs` keeps the value off screen.
#   2. Shell history    -- typed at a prompt, never in a command.
#   3. Process table    -- travels on ssh stdin, never in argv. Anything in argv
#                          is world-readable via `ps`.
#   4. Disk permissions -- .env is created under `umask 077` and chmod 600
#                          before the value is written.
#
# Usage:
#   ./scripts/set-secret.sh GOG_KEYRING_PASSWORD
#   ./scripts/set-secret.sh JOBCLAW_SHEET_ID
#
set -euo pipefail

readonly SSH_HOST="${JOBCLAW_SSH_HOST:-jobclaw}"
readonly REMOTE_USER="openclaw"
readonly TARGET_ENV="${JOBCLAW_ENV_FILE:-/home/openclaw/jobclaw/.env}"

if [[ $# -ne 1 ]]; then
    printf 'usage: %s <VARIABLE_NAME>\n' "$0" >&2
    exit 64
fi

readonly VAR_NAME="$1"

# Reject anything that is not a plausible environment variable name, since it is
# interpolated into the remote script.
if [[ ! "${VAR_NAME}" =~ ^[A-Z][A-Z0-9_]*$ ]]; then
    printf 'error: %q is not a valid variable name (expected UPPER_SNAKE_CASE)\n' \
        "${VAR_NAME}" >&2
    exit 64
fi

# Runs on the host as the openclaw user. Reads the value from stdin.
readonly REMOTE_SCRIPT='
set -euo pipefail

ENV_FILE="'"${TARGET_ENV}"'"
VAR_NAME="'"${VAR_NAME}"'"

IFS= read -r VALUE || true

if [ -z "${VALUE:-}" ]; then
    printf "error: no value received on stdin\n" >&2
    exit 1
fi

umask 077

if [ -f "$ENV_FILE" ]; then
    cp -p "$ENV_FILE" "$ENV_FILE.bak"
    grep -v "^${VAR_NAME}=" "$ENV_FILE" > "$ENV_FILE.tmp" || true
    mv "$ENV_FILE.tmp" "$ENV_FILE"
else
    : > "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
printf "%s=%s\n" "$VAR_NAME" "$VALUE" >> "$ENV_FILE"

printf "wrote %s to %s (mode %s, value length %s)\n" \
    "$VAR_NAME" \
    "$ENV_FILE" \
    "$(stat -c "%a" "$ENV_FILE")" \
    "${#VALUE}"
'

printf 'Setting %s on %s:%s\n\n' "${VAR_NAME}" "${SSH_HOST}" "${TARGET_ENV}"

printf 'Paste the value for %s (input hidden): ' "${VAR_NAME}"
IFS= read -rs SECRET_VALUE
printf '\n'

if [[ -z "${SECRET_VALUE}" ]]; then
    printf 'error: empty value, nothing written\n' >&2
    exit 1
fi

REMOTE_B64="$(printf '%s' "${REMOTE_SCRIPT}" | base64 | tr -d '\n')"
readonly REMOTE_B64

# Value on stdin; script base64-encoded in argv so quoting cannot break the
# remote parse. No heredoc: a heredoc would claim ssh's stdin and the value would
# never arrive.
if ! printf '%s\n' "${SECRET_VALUE}" | ssh "${SSH_HOST}" \
    "sudo -u ${REMOTE_USER} bash -c 'eval \"\$(printf %s ${REMOTE_B64} | base64 -d)\"'"; then
    printf 'error: failed to write remote env file\n' >&2
    unset SECRET_VALUE
    exit 1
fi

unset SECRET_VALUE

printf '\nDone. Verify it is visible without printing it:\n'
printf '  ssh %s '"'"'sudo -u %s bash -lc "cd /home/openclaw/jobclaw && set -a && . ./.env && set +a && test -n \\"\$%s\\" && echo VISIBLE"'"'"'\n' \
    "${SSH_HOST}" "${REMOTE_USER}" "${VAR_NAME}"
