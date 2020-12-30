#!/bin/bash

# A simple integration script to test the protected endpoint if scrapexchange.
#
# The scrapexchange server should be running locally, such as:
#
#    $ make start-local-server

# Create a scrap.
curl -d '{"Type":"svg", "Body": "<svg></svg>"}' -H 'Content-Type: application/json' http://localhost:9000/_/scraps/

# Retrieve scrap.
curl http://localhost:9000/_/scraps/svg/f7b0bac33b5f5b3ac86bec9f33c2d1c3ef025a9e4282ca7a8b9bc01e40d40556

# Create a name for the scrap.
curl -X PUT -d '{"Hash":"f7b0bac33b5f5b3ac86bec9f33c2d1c3ef025a9e4282ca7a8b9bc01e40d40556", "Description": "Testing"}' -H 'Content-Type: application/json' http://localhost:9000/_/names/svg/@smallest_svg

# List all named scraps.
curl http://localhost:9000/_/names/svg/

# Get scrap by name.
curl http://localhost:9000/_/names/svg/@smallest_svg

# Get the scrap templated in C++.
curl http://localhost:9000/_/tmpl/svg/@smallest_svg/cpp

printf "\n"

# Get raw scrap.
curl http://localhost:9000/_/raw/svg/@smallest_svg

printf "\n"

# Delete both the scrap and the name.
curl -X DELETE http://localhost:9000/_/scraps/svg/@smallest_svg

# Metrics
curl -s http://localhost:20000/metrics | grep scrap_exchange
