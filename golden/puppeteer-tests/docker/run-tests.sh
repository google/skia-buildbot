#!/bin/bash

# This script is designed to run inside the gold-puppeteer-tests Docker
# container.

# Populate /tests with all the files required to run the Puppeteer tests, which
# are copied from the buildbot repository checkout.
mkdir /tests/common-sk
cp -r /src/common-sk/package.json   /tests/common-sk
cp -r /src/common-sk/*.js           /tests/common-sk
cp -r /src/common-sk/modules        /tests/common-sk
cp -r /src/common-sk/plugins        /tests/common-sk
mkdir /tests/infra-sk
cp -r /src/infra-sk/package.json    /tests/infra-sk
cp -r /src/infra-sk/*.js            /tests/infra-sk
cp -r /src/infra-sk/modules         /tests/infra-sk
mkdir /tests/golden
cp -r /src/golden/package.json      /tests/golden
cp -r /src/golden/webpack.config.js /tests/golden
cp -r /src/golden/modules           /tests/golden
cp -r /src/golden/puppeteer-tests   /tests/golden

# Populate the various node_modules directories.
# TODO(lovisolo): Use "npm ci" once package-lock.json files are in repo.
cd /tests/common-sk
npm install
cd /tests/infra-sk
npm install
cd /tests/golden
npm install

# Run tests.
cd /tests/golden/puppeteer-tests
npx mocha
