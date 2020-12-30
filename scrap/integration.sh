#!/bin/bash

curl -d '{"Type":"svg", "Body": "<svg></svg>"}' -H 'Content-Type: application/json' http://localhost:9000/_/scraps/