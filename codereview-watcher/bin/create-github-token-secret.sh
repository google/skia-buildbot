#/bin/bash
# Creates the codereview-watcher-github-token secret.
set -e -x
if [ "$#" -ne 1 ]; then
  echo "The argument must be the github token."
  echo ""
  echo "./create-github-token-secret.sh xyz"
  exit 1
fi
SECRET_VALUE=$1
SECRET_NAME="codereview-watcher-github-token"
echo ${SECRET_VALUE} >> github_token

../../kube/secrets/add-secret-from-directory.sh \
  github_token \
  skia-public \
  ${SECRET_NAME}
