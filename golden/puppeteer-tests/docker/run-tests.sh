#!/bin/bash

# This script is designed to run inside the gold-puppeteer-tests Docker
# container.

set -x
# Populate /tests with all the files required to run the Puppeteer tests, which
# are copied from the buildbot repository checkout.
mkdir /tests/common-sk
cp -r /src/common-sk/package*       /tests/common-sk
cp -r /src/common-sk/*.js           /tests/common-sk
cp -r /src/common-sk/modules        /tests/common-sk
cp -r /src/common-sk/plugins        /tests/common-sk
mkdir /tests/infra-sk
cp -r /src/infra-sk/package*        /tests/infra-sk
cp -r /src/infra-sk/*.js            /tests/infra-sk
cp -r /src/infra-sk/modules         /tests/infra-sk
mkdir /tests/golden
cp -r /src/golden/package*          /tests/golden
cp -r /src/golden/webpack.config.js /tests/golden
cp -r /src/golden/modules           /tests/golden
cp -r /src/golden/puppeteer-tests   /tests/golden
cp -r /src/golden/demo-page-assets  /tests/golden

# Populate the various node_modules directories.
cd /tests/common-sk
npm ci
cd /tests/infra-sk
npm ci
cd /tests/golden
npm ci

# Run tests.
cd /tests/golden/puppeteer-tests
npx mocha
