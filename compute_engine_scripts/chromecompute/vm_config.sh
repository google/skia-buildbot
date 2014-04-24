# This file contains constants for the shell scripts which interact
# with the Skia chromecompute GCE instances.

GCUTIL=`which gcutil`

# Set all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)

# Reserving the following IPs for testing Cluster Telemetry changes.
# Cluster telemetry master: 108.170.192.0

# TODO(rmistry): Investigate moving the below constant to compute_engine_cfg.py
REQUIRED_FILES_FOR_SKIA_BOTS=(~/.autogen_svn_username \
                              ~/.autogen_svn_password \
                              ~/.skia_svn_username \
                              ~/.skia_svn_password \
                              ~/.boto)

GCOMPUTE_CMD="$GCUTIL --cluster=prod --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD --zone=$ZONE ssh --ssh_user=$PROJECT_USER"
