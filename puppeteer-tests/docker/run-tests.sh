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
curl -fsSL https://deb.nodesource.com/setup_14.x | bash
# Now when we install nodejs, it will use the v14 linked above.
apt-get install -y nodejs build-essential

npm install -g npm@7.21.0

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

mkdir /tests/puppeteer-tests
cp -r /src/puppeteer-tests/*.ts              /tests/puppeteer-tests
cp -r /src/puppeteer-tests/tsconfig.json     /tests/puppeteer-tests

################################################################################
# Install node modules and WASM dependencies.                                  #
################################################################################

cd /tests
# https://docs.npmjs.com/cli/v7/using-npm/config
npm ci --fetch-retry-maxtimeout 300000 --fetch-timeout 600000

################################################################################
# Run tests.                                                                   #
################################################################################

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
# https://github.com/webpack/webpack-sources/issues/66
export NODE_OPTIONS="--max-old-space-size=8192"

cd /tests/puppeteer-tests
npx mocha --require ts-node/register ./**/*_test.ts
