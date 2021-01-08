#!/bin/bash

# Create a single named SVG scrap @smiley that is a smiley face.
#
# The scrapexchange server should be running locally, such as:
#
#    $ make start-local-server
#
# Or the production protected endpoint can be forwarded to localhost:
#
#    $ kubectl port-forward service/scrapexchange 9000
#

# Create a scrap.
HASH=`curl --silent --data @./smile-svg-scrap.json -H 'Content-Type: application/json' http://localhost:9000/_/scraps/ | jq -r '.Hash'`

printf "Hash: ${HASH}\n"

# Create a name for the scrap.
curl --silent -X PUT -d "{\"Hash\": \"${HASH}\", \"Description\": \"Smiley Face\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/svg/@smiley

# List all named scraps.
curl --silent http://localhost:9000/_/names/svg/
