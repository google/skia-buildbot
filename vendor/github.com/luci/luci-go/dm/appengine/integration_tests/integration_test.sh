#!/usr/bin/env bash

# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

cd "${0%/*}"

: ${HOST:=:8080}
: ${VERBOSE:=1}
CFGVERS=1

DEVCFG_SYMLINK=$(dirname $PWD)/devcfg
DEVCFG_VERSIONS_DIR="$PWD/.devcfg_versions"

echo Using host: $HOST

_new_cfg_vers() {
  local oldVers=$CFGVERS
  CFGVERS=$(( $CFGVERS + 1 ))
  cp -a $DEVCFG_VERSIONS_DIR/$oldVers $DEVCFG_VERSIONS_DIR/$CFGVERS
}

_link_current_cfg() {
  rm -f $DEVCFG_SYMLINK
  ln -s $DEVCFG_VERSIONS_DIR/$CFGVERS $DEVCFG_SYMLINK
}

if [[ $1 == "remote" ]];
then
  echo "Non-local execution not currently supported"
  exit 1
else
  echo "Doing local test"
  rm -rf $DEVCFG_VERSIONS_DIR
  rm -f $DEVCFG_SYMLINK
  mkdir -p $DEVCFG_VERSIONS_DIR/$CFGVERS
  _link_current_cfg
fi

log() {
  echo $@ > /dev/stderr
}

write_config() {
  local outFile=$1
  _new_cfg_vers
  log "Writing config v$CFGVERS to: $outFile"
  mkdir -p $DEVCFG_VERSIONS_DIR/$CFGVERS/`dirname $outFile`
  cat > $DEVCFG_VERSIONS_DIR/$CFGVERS/$outFile
  _link_current_cfg
}

call() {
  local method=$1
  shift
  if [[ $VERBOSE == 1 ]]; then
    log "RPC[$HOST]" "$method($@)"
  fi
  rpc call $HOST dm.Deps.$method "$@"
}

add_attempts() {
  local cfgName=$1
  local payload=$2
  shift 2
  log "Adding quest: $@"
  local args=(
    -quest.json_payload "$payload"
    -quest.distributor_config_name "$cfgName"
  )
  id=$(dmtool hash -json_payload "$payload" -distributor_config_name "$cfgName")
  for attempt in $@; do
    args+=("-attempts.to.$id.nums" $attempt)
  done

  call EnsureGraphData "${args[@]}" > /dev/stderr
  echo $id
}

walk_attempt() {
  local questID=$1
  local attemptID=$2

  call WalkGraph -query.attempt_list.to.$questID.nums $attemptID \
    -limit.max_depth 100 -include.all
}
