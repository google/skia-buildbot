#!/bin/bash

# This script is designed to run inside the puppeteer-tests Docker container.

set -x

################################################################################
# Populate /tests with a subset of the buildbot repository that includes all   #
# Puppeteer tests and their dependencies. This is much faster than copying the #
# entire repository into the container.                                        #
#                                                                              #
# The buildbot repository should be mounted at /src.                           #
################################################################################

cp -r /src/.mocharc.json            /tests

mkdir /tests/common-sk
cp -r /src/common-sk/package*       /tests/common-sk
cp -r /src/common-sk/*.js           /tests/common-sk
cp -r /src/common-sk/modules        /tests/common-sk
cp -r /src/common-sk/plugins        /tests/common-sk

mkdir /tests/infra-sk
cp -r /src/infra-sk/package*        /tests/infra-sk
cp -r /src/infra-sk/*.js            /tests/infra-sk
cp -r /src/infra-sk/modules         /tests/infra-sk

mkdir /tests/puppeteer-tests
cp -r /src/puppeteer-tests/package* /tests/puppeteer-tests
cp -r /src/puppeteer-tests/*.js     /tests/puppeteer-tests

mkdir /tests/golden
cp -r /src/golden/package*          /tests/golden
cp -r /src/golden/webpack.config.js /tests/golden
cp -r /src/golden/modules           /tests/golden
cp -r /src/golden/demo-page-assets  /tests/golden

mkdir /tests/perf
cp -r /src/perf/package*            /tests/perf
cp -r /src/perf/webpack.config.js   /tests/perf
cp -r /src/perf/modules             /tests/perf

mkdir /tests/am
cp -r /src/am/package*              /tests/am
cp -r /src/am/webpack.config.js     /tests/am
cp -r /src/am/modules               /tests/am

################################################################################
# Install node modules.                                                        #
################################################################################

cd /tests/common-sk
npm ci

cd /tests/infra-sk
npm ci

cd /tests/puppeteer-tests
npm ci

cd /tests/golden
npm ci

cd /tests/perf
npm ci

cd /tests/am
npm ci

################################################################################
# Run tests.                                                                   #
################################################################################

cd /tests/puppeteer-tests
npx mocha .

cd /tests/golden
npx mocha ./**/*_puppeteer_test.js

cd /tests/perf
npx mocha ./**/*_puppeteer_test.js

cd /tests/am
npx mocha ./**/*_puppeteer_test.js

################################################################################
# Output screenshots in //puppeteer-tests/output are owned by the user on the  #
# Docker container running this script. Therefore we must make the screenshots #
# world-writable to avoid annoying permission errors when running the          #
# containerized tests locally during development.                              #
################################################################################

cd /out
chmod ugo+w *.png
