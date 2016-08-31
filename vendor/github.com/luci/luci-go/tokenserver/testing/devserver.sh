#!/bin/bash
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# devserver.sh launches local GAE dev server with the token server instance.

cd $(dirname $0)
. ./include.sh

cd ../appengine
gae.py devserver -A "$CLOUD_PROJECT_ID" -- \
  --port "$DEVSERVER_PORT" \
  --admin_port "$DEVSERVER_ADMIN_PORT" \
  --storage_path "$WORKING_DIR/gae_storage"
