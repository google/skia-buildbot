#!/bin/bash

# This script is designed to run inside the gold-puppeteer-tests Docker
# container.

# Populate /tests with all the files required to run the Puppeteer tests, which
# are copied from the buildbot repository checkout.
cp -r /src/golden/package.json      /tests
cp -r /src/golden/webpack.config.js /tests
cp -r /src/golden/modules           /tests
cp -r /src/golden/puppeteer-tests   /tests

# Populate /tests/node_modules.
cd /tests
npm install  # TODO(lovisolo): Use "npm ci" once package-lock.json is in repo.

# Run tests.
cd /tests/puppeteer-tests
npx mocha
