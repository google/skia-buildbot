#!/bin/bash

# This script takes ~2s to complete after the Docker image is built for the
# first time.

cd "$(dirname "$0")" # Set working directory to this script's directory.

cp dockerignore ../../.dockerignore
docker build -t gold-puppeteer-tests -f Dockerfile ../.. --quiet  # Remove --quiet for debugging.
rm ../../.dockerignore
