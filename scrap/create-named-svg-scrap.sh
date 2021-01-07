#!/bin/bash

# A simple integration script to test the protected endpoint if scrapexchange.
#
# The scrapexchange server should be running locally, such as:
#
#    $ make start-local-server

# Create a scrap.
curl --data @./smile-svg-scrap.json -H 'Content-Type: application/json' http://localhost:9000/_/scraps/

# Retrieve scrap.
curl http://localhost:9000/_/scraps/svg/733ddfb851e2ec8a73edaa6399b2110286c83e8b23a204b495b5910dbbd1be71

# Create a name for the scrap.
curl -X PUT -d '{"Hash":"733ddfb851e2ec8a73edaa6399b2110286c83e8b23a204b495b5910dbbd1be71", "Description": "Smiley Face"}' -H 'Content-Type: application/json' http://localhost:9000/_/names/svg/@smiley

# List all named scraps.
curl http://localhost:9000/_/names/svg/
