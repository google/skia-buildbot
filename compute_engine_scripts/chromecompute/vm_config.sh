# This file contains constants for the shell scripts which interact
# with the Skia chromecompute GCE instances.

GCUTIL=`which gcutil`

# Set all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)

# TODO(rmistry): Investigate moving the below constants to compute_engine_cfg.py
REQUIRED_FILES_FOR_LINUX_BOTS=(~/.skia_svn_username \
                               ~/.skia_svn_password \
                               ~/.boto)
REQUIRED_FILES_FOR_WIN_BOTS=(/tmp/chrome-bot.txt)

GCOMPUTE_CMD="$GCUTIL --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD --zone=$ZONE ssh --ssh_user=$PROJECT_USER"
