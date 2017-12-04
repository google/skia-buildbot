#!/bin/bash

set -x -e

# We start in infra/bots
cd ../..
export PATH=../cipd_bin_packages:../cipd_bin_packages/bin:$PATH
git init
git add .
git commit -m "Commit Recipes"
python infra/bots/recipes.py bundle --destination ${1}/recipe_bundle
