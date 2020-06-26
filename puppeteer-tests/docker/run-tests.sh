#!/bin/bash

# This script is designed to run inside the puppeteer-tests Docker container.

################################################################################
# Set up script environment.                                                   #
################################################################################

# This function will be called before the script exits.
#
# It pipes through the exit code of the last command executed, so that the
# tryjob responsible for running Puppeteer tests turns red upon a failing test
# case (or any other error.)
#
# It also makes the output screenshots world-writable to avoid permission errors
# during local development.
function cleanup {
  # Save the exit code of the last command executed.
  exit_code=$?

  # Output screenshots in //puppeteer-tests/output are owned by the user on the
  # Docker container running this script. Therefore we must make the screenshots
  # world-writable to avoid annoying permission errors when running the
  # containerized tests locally during development.
  cd /out
  chmod ugo+w *.png

  # Pipe through the exit code of the last command executed before this
  # function was called.
  exit $exit_code
}

# Print out any commands executed. This aids with debugging.
set -x

# End execution if a command returns a non-zero exit code.
set -e

# Execute the cleanup function defined above before exiting. This will happen
# both when a command returns a non-zero exit code and when this script finishes
# successfully.
trap cleanup EXIT

################################################################################
# Populate /tests with a subset of the buildbot repository that includes all   #
# Puppeteer tests and their dependencies. This is much faster than copying the #
# entire repository into the container.                                        #
#                                                                              #
# The buildbot repository should be mounted at /src.                           #
################################################################################

cp -r /src/.mocharc.json                 /tests

mkdir /tests/infra-sk
cp -r /src/infra-sk/package*             /tests/infra-sk
cp -r /src/infra-sk/*.ts                 /tests/infra-sk
cp -r /src/infra-sk/tsconfig.json        /tests/infra-sk
cp -r /src/infra-sk/modules              /tests/infra-sk
cp -r /src/infra-sk/pulito               /tests/infra-sk

mkdir /tests/puppeteer-tests
cp -r /src/puppeteer-tests/package*      /tests/puppeteer-tests
cp -r /src/puppeteer-tests/*.ts          /tests/puppeteer-tests
cp -r /src/puppeteer-tests/tsconfig.json /tests/puppeteer-tests

mkdir /tests/golden
cp -r /src/golden/package*               /tests/golden
cp -r /src/golden/webpack.config.ts      /tests/golden
cp -r /src/golden/tsconfig.json          /tests/golden
cp -r /src/golden/modules                /tests/golden
cp -r /src/golden/demo-page-assets       /tests/golden

mkdir /tests/perf
cp -r /src/perf/package*                 /tests/perf
cp -r /src/perf/webpack.config.ts        /tests/perf
cp -r /src/perf/tsconfig.json            /tests/perf
cp -r /src/perf/modules                  /tests/perf

mkdir /tests/am
cp -r /src/am/package*                   /tests/am
cp -r /src/am/webpack.config.ts          /tests/am
cp -r /src/am/tsconfig.json              /tests/am
cp -r /src/am/modules                    /tests/am

mkdir /tests/ct
cp -r /src/ct/package*                   /tests/ct
cp -r /src/ct/webpack.config.ts          /tests/ct
cp -r /src/ct/tsconfig.json              /tests/ct
cp -r /src/ct/modules                    /tests/ct

################################################################################
# Install node modules.                                                        #
################################################################################

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

cd /tests/ct
npm ci

################################################################################
# Run tests.                                                                   #
################################################################################

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
# https://github.com/webpack/webpack-sources/issues/66
export NODE_OPTIONS="--max-old-space-size=8192"

cd /tests/puppeteer-tests
npx mocha -r ts-node/register ./**/*_test.ts

# The ./**/*_puppeteer_test.ts glob patterns below exclude the Karma tests.

cd /tests/infra-sk
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/golden
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/perf
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/am
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/ct
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts
