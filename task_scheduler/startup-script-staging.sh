#! /bin/bash

# Create tmp dir if needed.
sudo -u default mkdir -p /mnt/pd0/tmp

# These must be set, otherwise bot_update will temporarily set them and then
# unset them after it has finished, which fails because we run bot_update
# concurrently. See also
# https://chromium-review.googlesource.com/c/chromium/tools/depot_tools/+/1036309
sudo -u default git config --global user.email \
  task-scheduler@skia-buildbots.google.com.iam.gserviceaccount.com
sudo -u default git config --global user.name skia-task-scheduler-staging
