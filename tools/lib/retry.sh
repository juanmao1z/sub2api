#!/usr/bin/env bash

retry_with_backoff() {
  local label=$1
  shift
  local attempts=${RETRY_ATTEMPTS:-4}
  local delay=${RETRY_DELAY_SECONDS:-5}
  local max_delay=${RETRY_MAX_DELAY_SECONDS:-30}
  local attempt=1
  local status=0

  case "$attempts" in
    ''|*[!0-9]*) echo "RETRY_ATTEMPTS must be a positive integer" >&2; return 2 ;;
  esac
  case "$delay" in
    ''|*[!0-9]*) echo "RETRY_DELAY_SECONDS must be a non-negative integer" >&2; return 2 ;;
  esac
  case "$max_delay" in
    ''|*[!0-9]*) echo "RETRY_MAX_DELAY_SECONDS must be a non-negative integer" >&2; return 2 ;;
  esac
  [ "$attempts" -ge 1 ] || { echo "RETRY_ATTEMPTS must be at least 1" >&2; return 2; }

  while :; do
    if "$@"; then
      return 0
    else
      status=$?
    fi

    if [ "$attempt" -ge "$attempts" ]; then
      printf '%s failed after %s attempt(s)\n' "$label" "$attempt" >&2
      return "$status"
    fi

    printf '%s failed on attempt %s/%s; retrying in %ss\n' \
      "$label" "$attempt" "$attempts" "$delay" >&2
    sleep "$delay"
    attempt=$((attempt + 1))
    if [ "$delay" -lt "$max_delay" ]; then
      delay=$((delay * 2))
      [ "$delay" -le "$max_delay" ] || delay=$max_delay
    fi
  done
}
