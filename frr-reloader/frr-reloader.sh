#!/bin/bash
cleanup() {
  echo "Caught an exit signal.."
  clean_files
  kill_sleep
  exit
}

reload_frr() {
  flock 200
  echo "Caught SIGHUP and acquired lock! Reloading FRR.."
  SECONDS=0

  kill_sleep

  echo "Checking the configuration file syntax"
  if ! python3 /usr/lib/frr/frr-reload.py --test --stdout "$FILE_TO_RELOAD" ; then
    echo "Syntax error spotted: aborting.. $SECONDS seconds"
    echo -n "$(date +%s) failure"  > "$STATUSFILE"
    return
  fi

  echo "Applying the configuration file"
  if ! python3 /usr/lib/frr/frr-reload.py --reload --overwrite --stdout "$FILE_TO_RELOAD" ; then
    echo "Failed to fully apply configuration file $SECONDS seconds"
    echo -n "$(date +%s) failure"  > "$STATUSFILE"
    return
  fi
  
  echo "FRR reloaded successfully! $SECONDS seconds"
  echo -n "$(date +%s) success"  > "$STATUSFILE"
} 200<"$LOCKFILE"

kill_sleep() {
  kill "$sleep_pid"
}

clean_files() {
  rm -f "$PIDFILE"
  rm -f "$LOCKFILE"
}

trap cleanup SIGTERM SIGINT
# The need for & is explained here: https://github.com/metallb/metallb/pull/935#issuecomment-943097999
# TLDR: & allows signals to trigger reload_frr immediately, flock keeps the order and creates a queue.
trap 'reload_frr &' HUP

SHARED_VOLUME="${SHARED_VOLUME:-/etc/frr_reloader}"
PIDFILE="$SHARED_VOLUME/reloader.pid"
FILE_TO_RELOAD="$SHARED_VOLUME/frr.conf"
LOCKFILE="$SHARED_VOLUME/lock"
STATUSFILE="$SHARED_VOLUME/.status"

clean_files
echo "PID is: $$, writing to $PIDFILE"
printf "$$" > "$PIDFILE"
touch "$LOCKFILE"

while true
do
    sleep infinity &
    sleep_pid=$!
    wait $sleep_pid 2>/dev/null
done
