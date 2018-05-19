#/bin/bash

# Displays all images that are out of date.

diff <(grep -h "image: " ./skia-public/*.yaml | sed "s/[ ]*image:[ ]*//" | sort | uniq) \
  <(kubectl get pods -o json | jq -r '..|.containerStatuses?|select(.!=null)|.[].image' | sort | uniq)
