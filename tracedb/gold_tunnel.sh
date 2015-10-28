#!/bin/bash

# Sets up an SSH port forwarding from localhost:9090 to skia-tracedb:10000,
# which is where the traceserver for Gold traces should be listening.
gcloud compute ssh default@skia-tracedb --zone=us-central1-c --ssh-flag="-L 9090:localhost:10000"
