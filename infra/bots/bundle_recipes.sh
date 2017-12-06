#!/bin/bash

set -x -e

export PATH=$(pwd)/cipd_bin_packages:$(pwd)/cipd_bin_packages/bin:$PATH
cd buildbot
git init
git add .
git commit -m "Commit Recipes"
python infra/bots/recipes.py bundle --destination ${1}/recipe_bundle
