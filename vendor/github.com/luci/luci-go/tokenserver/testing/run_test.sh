#!/bin/bash
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# run_test.sh runs the actual test.
#
# It assumes devserver.sh and crlserver.sh run in background already.

cd $(dirname $0)
. ./include.sh

echo "Building go code..."
go install -v github.com/luci/luci-go/client/cmd/rpc
go install -v github.com/luci/luci-go/tokenserver/cmd/luci_machine_tokend

function clean_tokens {
  local cert_name=$1
  rm -f "$WORKING_DIR/$cert_name.tok"
  rm -f "$WORKING_DIR/$cert_name.status"
}

function call_tokend {
  local cert_name=$1
  $GOBIN/luci_machine_tokend \
    -backend "localhost:$DEVSERVER_PORT" \
    -cert-pem "$CA_DIR/certs/$cert_name.pem" \
    -pkey-pem "$CA_DIR/private/$cert_name.pem" \
    -token-file "$WORKING_DIR/$cert_name.tok" \
    -status-file "$WORKING_DIR/$cert_name.status" \
    -ts-mon-endpoint "file://$WORKING_DIR/tsmon.txt"
}

function dump_status {
  local cert_name=$1
  echo "Status of luci_machine_tokend call:"
  echo "==================================="
  cat "$WORKING_DIR/$cert_name.status"
  echo
  echo "==================================="
}

function dump_token_file {
  local cert_name=$1
  echo "Token file:"
  echo "==================================="
  cat "$WORKING_DIR/$cert_name.tok"
  echo
  echo "==================================="
}

# Make a CA, feed its config to the token server.
echo "Initializing CA..."
initialize_ca
import_config
fetch_crl

# Create a machine certificate.
create_client_certificate luci-token-server-test-1.fake.domain

# Make a new token.
clean_tokens luci-token-server-test-1.fake.domain
call_tokend luci-token-server-test-1.fake.domain
ret=$?
dump_token_file luci-token-server-test-1.fake.domain
dump_status luci-token-server-test-1.fake.domain
if [ $ret -ne 0 ]
then
  echo "FAIL"
  exit 1
fi

# Revoke the cert, wait a bit (>100 ms) for CRL cache to expire.
revoke_client_certificate luci-token-server-test-1.fake.domain
fetch_crl
sleep 1

# Should fail now.
clean_tokens luci-token-server-test-1.fake.domain
call_tokend luci-token-server-test-1.fake.domain
ret=$?
dump_status luci-token-server-test-1.fake.domain
if [ $ret -eq 0 ]
then
  echo "FAIL, luci_machine_tokend should have failed with error"
  exit 1
else
  echo "SUCCESS! luci_machine_tokend failed as it should have"
fi
