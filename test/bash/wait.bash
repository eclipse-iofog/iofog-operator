#!/usr/bin/env bash

function waitCmdGrep() {
  local TIMEOUT="$1"
  local CMD="$2"
  local TXT="$3"
  local SECS=0
  local STATUS=1
  echo "$TXT"
  while [ "$SECS" -lt "$TIMEOUT" ] && [ "$STATUS" -ne 0 ]; do
    STATUS=0
    local LOGS=$(eval "$CMD")
    local FOUND=$(echo "$LOGS" | grep "$TXT")
    log "$TXT"
    log "$FOUND"
    if [ -z "$FOUND" ]; then
      STATUS=1
    fi
    log $STATUS
    if [ "$STATUS" -ne 0 ]; then
      let "SECS=SECS+1"
      sleep 1
    fi
  done
  [ "$STATUS" -eq 0 ]
  [ "$SECS" -lt "$TIMEOUT" ]
}
