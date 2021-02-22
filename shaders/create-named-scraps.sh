#!/bin/bash

# Create the named SkSL scraps.
#
# The scrapexchange server should be available locally, such as forwarding the
# the production protected endpoint to localhost:
#
#    $ kubectl port-forward service/scrapexchange 9000
#
# Note that this is in place of an Admin UI for the Scrap Exchange server. Once
# that is complete it should be the canconical way to create named scraps.

# Create a name for each scrap.
curl --silent -X PUT -d "{\"Hash\": \"26a447d730ba7afe4df7709b9079fbe2d592136dca60b8ddab7e1b56ea302791\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@inputs
curl --silent -X PUT -d "{\"Hash\": \"f9ae5d2b4d9b4f5f60ae47b46c034bee16290739831f490ad014c3ba93d13e46\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iResolution
curl --silent -X PUT -d "{\"Hash\": \"c56c6550edb52aff98320153ab05a2bcfa1f300e62a5401e37d16814aaabd618\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iTime
curl --silent -X PUT -d "{\"Hash\": \"4bca396ca53e90795bda2920a1002a7733149bfe6543eddfa1b803d187581a61\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iMouse
curl --silent -X PUT -d "{\"Hash\": \"bff9e3fba6621e7ad09b736968d048ac1b0ef4a19f33cbf236bdec189acf57cb\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@default
curl --silent -X PUT -d "{\"Hash\": \"ce6356effbe0586323b1d673861f36454ef67c51507c7b0678e9eba274531cd2\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iImage

# List all named sksl scraps.
curl --silent http://localhost:9000/_/names/sksl/
