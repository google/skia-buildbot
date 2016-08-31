#!/bin/bash
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# crlserver.sh launches a local HTTP server that serves certificate revocation
# list file to the token server.
#
# Its URL is specified as 'crl_url' in the token server config.

cd $(dirname $0)
. ./include.sh

# Default SimpleHTTPServer tries to bind to '0.0.0.0' and it triggers firewall
# warning on OS X.
SCRIPT="
import BaseHTTPServer as bhs, SimpleHTTPServer as shs;
srv = bhs.HTTPServer(('127.0.0.1', $CRLSERVER_PORT), shs.SimpleHTTPRequestHandler);
srv.serve_forever()
"

# Need to serve a parent of $CA_DIR, since $CA_DIR itself is recreated in
# tests (and server continues to server delete directory).
cd "$WORKING_DIR"
python -c "$SCRIPT"
