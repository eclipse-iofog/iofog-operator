#!/usr/bin/env bash

function waitCmdGrep() {
  local TIMEOUT="$1"
  local CMD="$2"
  shift 2
  local TXTS=$@
  local SECS=0
  local STATUS=1
  while [ "$SECS" -lt "$TIMEOUT" ] && [ "$STATUS" -ne 0 ]; do
    STATUS=0
    local LOGS=$(eval "$CMD")
    for TXT in ${TXTS[@]}; do
      local FOUND=$(echo "$LOGS" | grep "$TXT")
      log "$TXT"
      log "$FOUND"
      if [ -z "$FOUND" ]; then
        STATUS=1
      fi
      log $STATUS
    done
    if [ "$STATUS" -ne 0 ]; then
      let "SECS=SECS+1"
      sleep 1
    fi
  done
  [ "$STATUS" -eq 0 ]
  [ "$SECS" -lt "$TIMEOUT" ]
}
