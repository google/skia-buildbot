# This file contains constants for the shell scripts which interact
# with the telemetry Google Compute Engine instances.

GCUTIL=`which gcutil`

VM_RECREATESKPS_BOT_NAME=${VM_RECREATESKPS_BOT_NAME:="skia-recreate-skps-bot"}
VM_RECREATESKPS_BOT_IP_ADDRESS='108.170.192.101'

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# We flip the default one as required by PCRs in bigcluster.
ZONE_TAG=${ZONE_TAG:=b}
ZONE=us-central2-$ZONE_TAG

# The Project ID is found in the Compute tab of the dev console.
# https://cloud.google.com/console#c=p&pid=182615506979
PROJECT_ID='google.com:chromecompute'

# The user id which owns the server on the vm instance
PROJECT_USER="default"

GCOMPUTE_CMD="$GCUTIL --cluster=prod --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD --zone=$ZONE ssh --ssh_user=$PROJECT_USER"

REQUIRED_FILES_FOR_RECREATESKPS_BOT=(~/.autogen_svn_username \
                                     ~/.autogen_svn_password \
                                     ~/.skia_svn_username \
                                     ~/.skia_svn_password \
                                     ~/.boto)
