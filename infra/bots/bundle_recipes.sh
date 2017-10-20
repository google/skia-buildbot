#!/bin/bash

set -x -e

# We start in infra/bots
cd ../..
git init
git add .
git commit -m "Commit Recipes"
pwd
ls
python recipes.py bundle --destination ${1}/recipe_bundle
