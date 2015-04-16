#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-grandcentral Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

# Sets all constants in compute_engine_cfg.py as env variables.

$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# The name of instance where skia grandcentral is running on.
INSTANCE_NAME=skia-grandcentral
