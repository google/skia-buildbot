# This file contains constants for the shell scripts which interact
# with the cluster telemetry instances.

NUM_SLAVES=${NUM_SLAVES:=100}
NUM_WEBPAGES=${NUM_WEBPAGES:=1000000}
MAX_WEBPAGES_PER_PAGESET=${MAX_WEBPAGES_PER_PAGESET:=1}

ADMIN_EMAIL="rmistry@google.com"

# The user id which owns the server on the cluster telemetry machines.
PROJECT_USER="chrome-bot"

# Slave activity names.
CREATING_PAGESETS_ACTIVITY="CREATING_PAGESETS"
RECORD_WPR_ACTIVITY="RECORD_WPR"

