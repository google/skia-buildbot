#!/bin/bash

# Sets up an SSH port forwarding from localhost:8008 to skia-task-scheduler:8008,
# which is where the task scheduler remote db endpoint is running.
gcloud compute ssh default@skia-task-scheduler --zone=us-central1-c --ssh-flag="-L 8008:localhost:8008"
