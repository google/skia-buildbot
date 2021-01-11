#!/bin/bash


# cat input.json |\
#   jq  '.json | tostring' |\
#   jq --raw-input '{ "Type": "particle", "Body": . }' |\
#   curl --silent --data @- -H 'Content-Type: application/json' http://localhost:9000/_/scraps/ |\
#   jq -r '.Hash'

for name in $(gsutil ls gs://skparticles-renderer/); do
   printf "%s -> " ${name}
   gsutil cat ${name}input.json |\
   jq  '.json | tostring' |\
   jq --raw-input '{ "Type": "particle", "Body": . }' |\
   curl --silent --data @- -H 'Content-Type: application/json' http://localhost:9000/_/scraps/ |\
   jq -r '.Hash'
done
