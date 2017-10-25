#!/bin/bash

# Wrapper for vulcanize which works around the fact that vulcanize writes to
# stdout, ensures that we exit with non-zero code if anything fails, and avoids
# writing to res/vul/elements.html until everything else is successful.

set -e

tmpfile=res/vul/elements.html.plzminify
mkdir -p res/vul
./node_modules/.bin/vulcanize --inline-css --inline-scripts --strip-comments --abspath=./ elements.html > $tmpfile
./node_modules/.bin/html-minifier -o res/vul/elements.html --minify-js --remove-comments --collapse-whitespace --conservative-collapse $tmpfile
rm $tmpfile
