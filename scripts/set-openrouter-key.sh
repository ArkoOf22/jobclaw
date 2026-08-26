#!/usr/bin/env bash
#
# Set OPENROUTER_API_KEY in the .env on the EC2 build host.
#
# Run this yourself, in your own terminal. Do not paste the key into an agent
# chat: on Kiro Free Tier or an individual subscription, conversation content may
# be used for service improvement, and Free Tier inputs are retained up to 60
# days for abuse detection. A secret in a transcript is a secret in a log.
#
# This script is careful about four leak paths:
#   1. Terminal echo    -- `read -rs` keeps the key off screen.
#   2. Shell history    -- the key is typed at a prompt, never in a command.
#   3. Process table    -- the key travels on ssh stdin, never in argv. Anything
#                          in argv is world-readable via `ps`, which is exactly
#                          how this project's gog tokens leak today. The remote
#                          script itself is passed as an argument, which is fine
#                          because the script is not secret.
#   4. Disk permissions -- the remote .env is created under `umask 077` before
#                          the key is written, so there is no window where it
#                          exists world-readable.
#
# Usage:
#   ./scripts/set-openrouter-key.sh
#
set -euo pipefail

readonly SSH_HOST="${JOBCLAW_SSH_HOST:-jobclaw}"
readonly REMOTE_USER="openclaw"
readonly REMOTE_DIR="/home/openclaw/jobclaw"

# Override for testing against a scratch path. Defaults to the real .env.
readonly TARGET_ENV="${JOBCLAW_ENV_FILE:-/home/openclaw/jobclaw/.env}"

# Runs on the host, as the openclaw user. Reads the key from stdin.
# Single-quoted so nothing is expanded locally except the target path.
readonly REMOTE_SCRIPT='
set -euo pipefail

ENV_FILE="'"${TARGET_ENV}"'"

IFS= read -r KEY || true

if [ -z "${KEY:-}" ]; then
    printf "error: no key received on stdin\n" >&2
    exit 1
fi

umask 077

if [ -f "$ENV_FILE" ]; then
    cp -p "$ENV_FILE" "$ENV_FILE.bak"
    grep -v "^OPENROUTER_API_KEY=" "$ENV_FILE" > "$ENV_FILE.tmp" || true
    mv "$ENV_FILE.tmp" "$ENV_FILE"
else
    : > "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
printf "OPENROUTER_API_KEY=%s\n" "$KEY" >> "$ENV_FILE"

# Report shape only, never the value.
printf "wrote %s (mode %s, %s bytes, key length %s)\n" \
    "$ENV_FILE" \
    "$(stat -c "%a" "$ENV_FILE")" \
    "$(stat -c "%s" "$ENV_FILE")" \
    "${#KEY}"
'

printf 'Setting OPENROUTER_API_KEY on %s:%s\n\n' \
    "${SSH_HOST}" "${TARGET_ENV}"

# -s: no echo. -r: no backslash mangling.
printf 'Paste your OpenRouter API key (input hidden): '
IFS= read -rs OPENROUTER_KEY
printf '\n'

if [[ -z "${OPENROUTER_KEY}" ]]; then
    printf 'error: empty key, nothing written\n' >&2
    exit 1
fi

# OpenRouter keys are prefixed sk-or-. Warn rather than fail, in case the format
# changes.
if [[ "${OPENROUTER_KEY}" != sk-or-* ]]; then
    printf 'warning: key does not start with "sk-or-". Continuing.\n' >&2
fi

# The script is base64-encoded so no amount of quoting in it can break the
# remote shell parse. It is decoded and eval'd on the far side.
#
# Critically: no heredoc. A heredoc would take over ssh's stdin and the key would
# never arrive. The key is the only thing on stdin.
REMOTE_B64="$(printf '%s' "${REMOTE_SCRIPT}" | base64 | tr -d '\n')"
readonly REMOTE_B64

if ! printf '%s\n' "${OPENROUTER_KEY}" | ssh "${SSH_HOST}" \
    "sudo -u ${REMOTE_USER} bash -c 'eval \"\$(printf %s ${REMOTE_B64} | base64 -d)\"'"; then
    printf 'error: failed to write remote env file\n' >&2
    unset OPENROUTER_KEY
    exit 1
fi

unset OPENROUTER_KEY

printf '\nDone. Confirm the app sees it without printing the value:\n'
printf '  ssh %s '"'"'sudo -u %s bash -lc "cd %s && set -a && . ./.env && set +a && test -n \\"\$OPENROUTER_API_KEY\\" && echo KEY_VISIBLE"'"'"'\n' \
    "${SSH_HOST}" "${REMOTE_USER}" "${REMOTE_DIR}"
