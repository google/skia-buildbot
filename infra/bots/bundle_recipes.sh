#!/bin/bash

set -x -e

# We start in infra/bots
cd ../..
ls ../cipd_bin_packages
ls -alh ../cipd_bin_packages/bin
PATH=../cipd_bin_packages:$PATH
which git
git --version
git init
git add .
git commit -m "Commit Recipes"
pwd
ls
python infra/bots/recipes.py bundle --destination ${1}/recipe_bundle
