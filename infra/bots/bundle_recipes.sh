#!/bin/bash

set -x -e

# We start in infra/bots
cd ../..
ls ../cipd_bin_packages
ls -alh ../cipd_bin_packages/bin
../cipd_bin_packages/bin/git --version
export INFRA_GIT_WRAPPER_TRACE=1
export PATH=../cipd_bin_packages:../cipd_bin_packages/bin:$PATH
which git
git --version
git init
git add .
git commit -m "Commit Recipes"
pwd
ls
python infra/bots/recipes.py bundle --destination ${1}/recipe_bundle
