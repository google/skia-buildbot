# This file contains constants for the shell scripts which interact
# with the Google Compute Engine instances.

GCUTIL=`which gcutil`

# The base names of the VM instances. Actual names are VM_NAME_BASE+name+zone
VM_NAME_BASE=${VM_NAME_BASE:='skia'}

VM_MASTER_NAMES=${VM_MASTER_NAMES:="master private-master"}
VM_SLAVE_NAMES=${VM_SLAVE_NAMES:="housekeeping-slave"}
VM_NAMES="${VM_MASTER_NAMES} ${VM_SLAVE_NAMES}"


REQUIRED_FILES_FOR_SLAVES=(~/.autogen_svn_username \
                           ~/.autogen_svn_password \
                           ~/.skia_svn_username \
                           ~/.skia_svn_password)

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# We flip the default one as required by PCRs in bigcluster. We are allowed
# us-east-a and us-east-b.
# A short tag to use as part of the VM instance name
ZONE_TAG=${ZONE_TAG:=a}
ZONE=us-east-$ZONE_TAG

# The Project ID is found in the Compute tab of the dev console.
# https://code.google.com/apis/console/?pli=1#project:31977622648:overview
PROJECT_ID='google.com:skia-buildbots'

# The user id which owns the server on the vm instance
PROJECT_USER="default"

GCOMPUTE_CMD="$GCUTIL --clustermanager=acquisitions --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCUTIL --clustermanager=acquisitions \
  --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER"
