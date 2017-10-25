#!/bin/bash

# Wrapper for vulcanize with makes things a bit saner.

set -e

tmpfile=res/vul/elements.html.plzminify
mkdir -p res/vul
./node_modules/.bin/vulcanize --inline-css --inline-scripts --strip-comments --abspath=./ elements.html > $tmpfile
./node_modules/.bin/html-minifier -o res/vul/elements.html --minify-js --remove-comments --collapse-whitespace --conservative-collapse $tmpfile
rm $tmpfile
