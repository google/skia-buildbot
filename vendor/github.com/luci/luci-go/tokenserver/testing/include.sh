#!/bin/bash
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# includes.sh is included by all other scripts.
#
# It contains a bunch of global variables and functions.


# Change to your Cloud Project ID. See README.md.
CLOUD_PROJECT_ID=my-cloud-project


WORKING_DIR=/tmp/token_server_test
CA_DIR=$WORKING_DIR/ca
CA_NAME="Fake CA: fake.ca"

mkdir -p "$WORKING_DIR"

DEVSERVER_PORT=8080
DEVSERVER_ADMIN_PORT=8100
CRLSERVER_PORT=8200

DEVCFG_PATH=`dirname $PWD`/appengine/devcfg/services/$CLOUD_PROJECT_ID


# initialize_ca builds a new simple self-signed CA.
#
# See https://jamielinux.com/docs/openssl-certificate-authority/
function initialize_ca {
  rm -rf "$CA_DIR"

  mkdir "$CA_DIR"
  mkdir "$CA_DIR/certs"
  mkdir "$CA_DIR/crl"
  mkdir "$CA_DIR/csr"
  mkdir "$CA_DIR/newcerts"
  mkdir "$CA_DIR/private"

  touch "$CA_DIR/index.txt"
  echo 1000 > "$CA_DIR/serial"
  echo 1000 > "$CA_DIR/crlnumber"

  cat > "$CA_DIR/openssl.cnf" <<EOL
[ca]
default_ca = CA_default

[CA_default]
dir               = $CA_DIR
certs             = $CA_DIR/certs
crl_dir           = $CA_DIR/crl
new_certs_dir     = $CA_DIR/newcerts
database          = $CA_DIR/index.txt
serial            = $CA_DIR/serial
RANDFILE          = $CA_DIR/private/.rand

# The root key and root certificate.
private_key       = $CA_DIR/private/ca.pem
certificate       = $CA_DIR/certs/ca.pem

# For certificate revocation lists.
crlnumber         = $CA_DIR/crlnumber
crl               = $CA_DIR/crl/crl.pem
crl_extensions    = crl_ext
default_crl_days  = 30

default_md        = sha256
name_opt          = ca_default
cert_opt          = ca_default
default_days      = 375
preserve          = no
policy            = policy_loose

[policy_loose]
countryName             = optional
stateOrProvinceName     = optional
localityName            = optional
organizationName        = optional
organizationalUnitName  = optional
commonName              = supplied
emailAddress            = optional

[req]
default_bits        = 2048
distinguished_name  = req_distinguished_name
string_mask         = utf8only
default_md          = sha256
x509_extensions     = v3_ca

[req_distinguished_name]
countryName                     = Country Name (2 letter code)
stateOrProvinceName             = State or Province Name
localityName                    = Locality Name
0.organizationName              = Organization Name
organizationalUnitName          = Organizational Unit Name
commonName                      = Common Name
emailAddress                    = Email Address

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, cRLSign, keyCertSign

[client_cert]
basicConstraints = CA:FALSE
nsCertType = client, email
nsComment = "OpenSSL Generated Client Certificate"
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
keyUsage = critical, nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, emailProtection

[crl_ext]
authorityKeyIdentifier=keyid:always
EOL

  # Create the root key pair.
  openssl genrsa -out "$CA_DIR/private/ca.pem" 2048

  # Create the root (self-signed) certificate.
  openssl req -config "$CA_DIR/openssl.cnf" \
    -key "$CA_DIR/private/ca.pem" \
    -new -x509 -days 7300 -sha256 -extensions v3_ca \
    -out "$CA_DIR/certs/ca.pem" \
    -subj "/C=US/ST=California/L=Blah/O=Stuff Inc/CN=$CA_NAME"

  # Generate first CRL.
  regen_crl
}


# create_client_certificate creates a new client key pair and signs the cert.
#
# Uses CA initialized with initialize_ca.
function create_client_certificate {
  local name=$1

  # Generate a key pair.
  openssl genrsa -out "$CA_DIR/private/$name.pem" 2048

  # Generate a certificate signing request.
  openssl req -config "$CA_DIR/openssl.cnf" \
    -key "$CA_DIR/private/$name.pem" \
    -new -sha256 -out "$CA_DIR/csr/$name.pem" \
    -subj "/C=US/ST=California/L=Blah/O=Stuff Inc/CN=$name"

  # Ask CA to sign the certificate.
  openssl ca -batch -config "$CA_DIR/openssl.cnf" \
    -extensions client_cert -days 375 -notext -md sha256 \
    -in "$CA_DIR/csr/$name.pem" \
    -out "$CA_DIR/certs/$name.pem"

  regen_crl
}

# revoke_client_certificate revokes previously issued certificate.
#
# Uses CA initialized with initialize_ca.
function revoke_client_certificate {
  local name=$1

  openssl ca -config "$CA_DIR/openssl.cnf" -revoke "$CA_DIR/certs/$name.pem"
  regen_crl
}


# regen_crl regenerates certificate revocation list file.
function regen_crl {
  openssl ca -config "$CA_DIR/openssl.cnf" -gencrl -out "$CA_DIR/crl/crl.pem"
  openssl crl -outform der -in "$CA_DIR/crl/crl.pem" -out "$CA_DIR/crl/crl.der"
}


# call_rpc invokes pRPC method on devserver instance.
#
# It reads method body as JSON from stdin.
function call_rpc {
  echo "Calling $1..."
  rpc call -format json "localhost:$DEVSERVER_PORT" $1
  if [ $? -ne 0 ]
  then
    echo "RPC call $1 failed!"
    exit 1
  fi
}


# import_config imports CA config into the token server.
function import_config {
  mkdir -p $DEVCFG_PATH/certs
  cp $CA_DIR/certs/ca.pem $DEVCFG_PATH/certs/ca.pem

  cat >$DEVCFG_PATH/tokenserver.cfg <<EOL
certificate_authority {
  cn: "$CA_NAME"
  cert_path: "certs/ca.pem"
  crl_url: "http://localhost:$CRLSERVER_PORT/ca/crl/crl.der"
  use_oauth: false

  known_domains: {
    domain: "fake.domain"
    machine_token_lifetime: 3600
  }
}
EOL

  # Ask the server to reread the config.
  echo "{}" | call_rpc "tokenserver.admin.CertificateAuthorities.ImportConfig"

  # Wait a bit for cached config to expire.
  sleep 0.5
}


# fetch_crl imports current CRL into the token server.
function fetch_crl {
  call_rpc "tokenserver.admin.CertificateAuthorities.FetchCRL" <<EOL
  {
    "cn": "$CA_NAME",
    "force": true
  }
EOL
}
