# This file contains constants for the shell scripts which interact with the
# Skia's GCE instances.

GCUTIL=`which gcutil`

# Set all constants in compute_engine_cfg.py as env variables.
$(python ../compute_engine_cfg.py)
if [ $? != "0" ]; then
  echo "Failed to read compute_engine_cfg.py!"
  exit 1
fi

# If this is true, then the VM instances will be set up with auth scopes
# appropriate for the android merge bot.
if [ "$VM_IS_ANDROID_MERGE" = 1 ]; then
  SCOPES="https://www.googleapis.com/auth/androidbuild.internal,https://www.googleapis.com/auth/gerritcodereview,$SCOPES"
fi

# TODO(rmistry): Investigate moving the below constants to compute_engine_cfg.py
CHROME_MASTER_HOST=/tmp/chrome_master_host
REQUIRED_FILES_FOR_LINUX_BOTS=(/tmp/.boto \
                               /tmp/.bot_password \
                               /tmp/.netrc \
                               $CHROME_MASTER_HOST)
REQUIRED_FILES_FOR_WIN_BOTS=(/tmp/chrome-bot.txt \
                             /tmp/.boto \
                             /tmp/.bot_password \
                             /tmp/.netrc \
                             $CHROME_MASTER_HOST)

GCOMPUTE_CMD="$GCUTIL --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD --zone=$ZONE ssh --ssh_user=$PROJECT_USER"
