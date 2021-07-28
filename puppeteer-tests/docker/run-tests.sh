#!/bin/bash

# This script is designed to run inside the Skia Infra RBE toolchain container.

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
# Prepare the RBE toolchain container to run Webpack-built Puppeteer tests.    #
################################################################################

# Install Node.js and build-essential, which is needed by some NPM packages.
curl -fsSL https://deb.nodesource.com/setup_15.x | bash
apt-get install -y nodejs build-essential

# Input/output directories.
mkdir -p /tests
mkdir -p /src

################################################################################
# Populate /tests with a subset of the buildbot repository that includes all   #
# Puppeteer tests and their dependencies. This is much faster than copying the #
# entire repository into the container.                                        #
#                                                                              #
# The buildbot repository should be mounted at /src.                           #
################################################################################

cp -r /src/.mocharc.json                     /tests
cp -r /src/package.json                      /tests
cp -r /src/package-lock.json                 /tests
cp -r /src/tsconfig.json                     /tests
cp -r /src/modules                           /tests

mkdir /tests/make
cp -r /src/make/npm.mk                       /tests/make

mkdir /tests/infra-sk
cp -r /src/infra-sk/*.ts                     /tests/infra-sk
cp -r /src/infra-sk/*.scss                   /tests/infra-sk
cp -r /src/infra-sk/tsconfig.json            /tests/infra-sk
cp -r /src/infra-sk/modules                  /tests/infra-sk
cp -r /src/infra-sk/pulito                   /tests/infra-sk

mkdir /tests/puppeteer-tests
cp -r /src/puppeteer-tests/*.ts              /tests/puppeteer-tests
cp -r /src/puppeteer-tests/tsconfig.json     /tests/puppeteer-tests

mkdir /tests/perf
cp -r /src/perf/webpack.config.ts            /tests/perf
cp -r /src/perf/tsconfig.json                /tests/perf
cp -r /src/perf/modules                      /tests/perf

mkdir /tests/am
cp -r /src/am/webpack.config.ts              /tests/am
cp -r /src/am/tsconfig.json                  /tests/am
cp -r /src/am/modules                        /tests/am

mkdir /tests/bugs-central
cp -r /src/bugs-central/webpack.config.ts    /tests/bugs-central
cp -r /src/bugs-central/tsconfig.json        /tests/bugs-central
cp -r /src/bugs-central/modules              /tests/bugs-central

mkdir /tests/ct
cp -r /src/ct/webpack.config.ts              /tests/ct
cp -r /src/ct/tsconfig.json                  /tests/ct
cp -r /src/ct/modules                        /tests/ct

mkdir /tests/new_element
cp -r /src/new_element/webpack.config.ts     /tests/new_element
cp -r /src/new_element/tsconfig.json         /tests/new_element
cp -r /src/new_element/modules               /tests/new_element

mkdir /tests/fiddlek
cp -r /src/fiddlek/webpack.config.ts         /tests/fiddlek
cp -r /src/fiddlek/tsconfig.json             /tests/fiddlek
cp -r /src/fiddlek/modules                   /tests/fiddlek

mkdir /tests/status
cp -r /src/status/webpack.config.ts          /tests/status
cp -r /src/status/tsconfig.json              /tests/status
cp -r /src/status/modules                    /tests/status

mkdir /tests/task_scheduler
cp -r /src/task_scheduler/webpack.config.ts  /tests/task_scheduler
cp -r /src/task_scheduler/tsconfig.json      /tests/task_scheduler
cp -r /src/task_scheduler/modules            /tests/task_scheduler

mkdir /tests/debugger-app
cp -r /src/debugger-app/webpack.config.ts    /tests/debugger-app
cp -r /src/debugger-app/tsconfig.json        /tests/debugger-app
cp -r /src/debugger-app/modules              /tests/debugger-app
cp -r /src/debugger-app/build                /tests/debugger-app
cp -r /src/debugger-app/static               /tests/debugger-app

mkdir /tests/scrap
cp -r /src/scrap/webpack.config.ts           /tests/scrap
cp -r /src/scrap/tsconfig.json               /tests/scrap
cp -r /src/scrap/modules                     /tests/scrap

mkdir /tests/particles
cp -r /src/particles/webpack.config.ts       /tests/particles
cp -r /src/particles/tsconfig.json           /tests/particles
cp -r /src/particles/modules                 /tests/particles
cp -r /src/particles/Makefile                /tests/particles

mkdir /tests/shaders
cp -r /src/shaders/webpack.config.ts         /tests/shaders
cp -r /src/shaders/tsconfig.json             /tests/shaders
cp -r /src/shaders/modules                   /tests/shaders
cp -r /src/shaders/Makefile                  /tests/shaders

mkdir /tests/machine
cp -r /src/machine/webpack.config.ts         /tests/machine
cp -r /src/machine/tsconfig.json             /tests/machine
cp -r /src/machine/modules                   /tests/machine

mkdir /tests/skcq
cp -r /src/skcq/webpack.config.ts         /tests/skcq
cp -r /src/skcq/tsconfig.json             /tests/skcq
cp -r /src/skcq/modules                   /tests/skcq

################################################################################
# Install node modules and WASM dependencies.                                  #
################################################################################

cd /tests
npm ci

cd /tests/particles
make wasm_libs_fixed

cd /tests/shaders
make wasm_libs_fixed

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

cd /tests/perf
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/am
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/bugs-central
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/ct
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/new_element
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/fiddlek
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/status
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/task_scheduler
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/debugger-app
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/scrap
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/particles
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/shaders
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/machine
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

cd /tests/skcq
npx mocha -r ts-node/register ./**/*_puppeteer_test.ts
