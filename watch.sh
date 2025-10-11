#!/bin/bash
READLINK=$(which greadlink >/dev/null 2>&1 && echo greadlink || echo readlink)
fswatch_pid_file="$($READLINK -f "fswatch.pid")"

unset app_pid
unset fswatch_pid
unset stopping

kill_app() {
  if [ -n "$app_pid" ]; then
    kill -9 $app_pid && echo "killed app with pid $app_pid"
  fi
}

cleanup() {
  stopping=true
  kill_app
  if [ -n "$fswatch_pid" ]; then
    if kill -9 $fswatch_pid; then
      echo "killed fswatch with pid $fswatch_pid";
      rm $fswatch_pid_file
    fi
  fi
}

trap cleanup EXIT SIGINT SIGTERM

build_and_run() {
  task plain
  LOG_LEVEL=debug \
    XCALIAPP_USERNAME="peter.dunay.kovacs@gmail.com" \
    SERVER_PORT=8888 \
    XCALIAPP_DRAWINGREPO_LIST="xcaliapp:XCalidraw Application,wsgw:WebSocket Gateway" \
    XCALIAPP_DRAWINGREPO_xcaliapp_STORETYPE=LOCAL_GIT \
    XCALIAPP_DRAWINGREPO_xcaliapp_ROOT=${HOME}/workspace/xcaliapp/main \
    XCALIAPP_DRAWINGREPO_xcaliapp_PATH=doc/design/diagrams \
    XCALIAPP_DRAWINGREPO_wsgw_STORETYPE=LOCAL_GIT \
    XCALIAPP_DRAWINGREPO_wsgw_ROOT=${HOME}/github/pdkovacs/wsgw \
    XCALIAPP_DRAWINGREPO_wsgw_PATH=doc/design/diagrams \
    ./xcaliapp-backend &
  app_pid=$!
}

while true; do
  kill_app
  build_and_run

  set +x
  fswatch -r -1 --event Created --event Updated --event Removed \
      -e '.*/[.]git/.*' \
      -e "$fswatch_pid_file"'$' \
      . &
  set +x
  fswatch_pid=$!
  echo $fswatch_pid >"$fswatch_pid_file"
  wait $fswatch_pid

  [[ "$stopping" == "true" ]] && exit
done 
