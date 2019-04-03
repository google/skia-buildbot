#/bin/bash

# Downloads the secret login.json file for editing, i.e. to change the salt,
# client_id or client_secret.
#
# Once you are done editing the file, the config will be uploaded.
SECRET_NAME=skia-corp-legacy-login-secrets
source ../bash/ramdisk.sh
source ../kube/corp-config.sh
cd /tmp/ramdisk
kubectl get secret ${SECRET_NAME} -o json | jq -r '.data["login.json"]' | base64 -d > login.json

echo "Downloaded the login secrets to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/login.json."
echo ""
read -r -p "When you are done editing press enter to upload the updated login.json file..." key
kubectl create secret generic ${SECRET_NAME} --from-file=login.json --dry-run -o yaml | kubectl apply -f -
cd -
