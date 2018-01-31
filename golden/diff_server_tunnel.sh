#!/bin/bash

# Sets up an SSH port forwarding from skia-diffserver-prod.
gcloud compute ssh default@skia-diffserver-prod --zone=us-central1-c --ssh-flag="-L 9100:localhost:8000 -L  9101:localhost:8001"
