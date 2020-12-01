#!/bin/bash

set -x -e

git init
git add .
git commit -m "Commit Recipes"
python infra/bots/recipes.py bundle --destination ${1}/recipe_bundle
