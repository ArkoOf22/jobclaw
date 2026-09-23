#!/usr/bin/env bash
#
# Set GEMINI_API_KEY in the .env on the EC2 build host.
#
# Run this yourself, in your own terminal. Do not paste the key into an agent
# chat: on Kiro Free Tier or an individual subscription, conversation content may
# be used for service improvement, and Free Tier inputs are retained up to 60
# days for abuse detection. A secret in a transcript is a secret in a log.
#
# The key MUST be a PAID-TIER AI Studio key. JobClaw sends resume and
# questionnaire prompts carrying the candidate's full history to Google, and
# Google's API has no per-request no-training flag. Only the paid tier is
# excluded from product-improvement training. See config/resume.yaml and the
# internal/llm/google package comment.
#
# This script is careful about four leak paths:
#   1. Terminal echo    -- `read -rs` keeps the key off screen.
#   2. Shell history    -- the key is typed at a prompt, never in a command.
#   3. Process table    -- the key travels on ssh stdin, never in argv. Anything
#                          in argv is world-readable via `ps`.
#   4. Disk permissions -- the remote .env is chmod 600 before the key is
#                          written, so there is no world-readable window.
#
# Usage:
#   ./scripts/set-gemini-key.sh
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
    grep -v "^GEMINI_API_KEY=" "$ENV_FILE" > "$ENV_FILE.tmp" || true
    mv "$ENV_FILE.tmp" "$ENV_FILE"
else
    : > "$ENV_FILE"
fi

chmod 600 "$ENV_FILE"
printf "GEMINI_API_KEY=%s\n" "$KEY" >> "$ENV_FILE"

# Report shape only, never the value.
printf "wrote %s (mode %s, %s bytes, key length %s)\n" \
    "$ENV_FILE" \
    "$(stat -c "%a" "$ENV_FILE")" \
    "$(stat -c "%s" "$ENV_FILE")" \
    "${#KEY}"
'

printf 'Setting GEMINI_API_KEY on %s:%s\n\n' \
    "${SSH_HOST}" "${TARGET_ENV}"

# -s: no echo. -r: no backslash mangling.
printf 'Paste your Gemini API key (input hidden): '
IFS= read -rs GEMINI_KEY
printf '\n'

if [[ -z "${GEMINI_KEY}" ]]; then
    printf 'error: empty key, nothing written\n' >&2
    exit 1
fi

# AI Studio keys are prefixed AIza. Warn rather than fail, in case the format
# changes.
if [[ "${GEMINI_KEY}" != AIza* ]]; then
    printf 'warning: key does not start with "AIza". Continuing.\n' >&2
fi

# The script is base64-encoded so no amount of quoting in it can break the
# remote shell parse. It is decoded and eval'd on the far side.
#
# Critically: no heredoc. A heredoc would take over ssh's stdin and the key would
# never arrive. The key is the only thing on stdin.
REMOTE_B64="$(printf '%s' "${REMOTE_SCRIPT}" | base64 | tr -d '\n')"
readonly REMOTE_B64

if ! printf '%s\n' "${GEMINI_KEY}" | ssh "${SSH_HOST}" \
    "sudo -u ${REMOTE_USER} bash -c 'eval \"\$(printf %s ${REMOTE_B64} | base64 -d)\"'"; then
    printf 'error: failed to write remote env file\n' >&2
    unset GEMINI_KEY
    exit 1
fi

unset GEMINI_KEY

printf '\nDone. Confirm the app sees it without printing the value:\n'
printf '  ssh %s '"'"'sudo -u %s bash -lc "cd %s && set -a && . ./.env && set +a && test -n \\"\$GEMINI_API_KEY\\" && echo KEY_VISIBLE"'"'"'\n' \
    "${SSH_HOST}" "${REMOTE_USER}" "${REMOTE_DIR}"
