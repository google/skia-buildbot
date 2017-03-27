#!/bin/bash

set -x -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source $SCRIPT_DIR/common.sh

PRE=`pwd`/out
# PRE=/usr/local
mkdir -p $PRE
PKG_CONFIG_PATH=${PRE}/
build_install $PRE
