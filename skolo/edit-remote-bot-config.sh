#/bin/bash

# Downloads the remote-bot-config.json secret files for editing.
#
# Once you are done editing the file, the config will be uploaded.
SECRET_NAME=remote-bot-config
source ../bash/ramdisk.sh
source ../kube/clusters.sh

__skia_rpi2

cd /tmp/ramdisk

kubectl get secret ${SECRET_NAME} -o json | jq -r '.data["remote-bot-config.json"]' | base64 -d > remote-bot-config.json

echo "Downloaded the secrets to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/remote-bot-config.json"
echo ""
read -r -p "When you are done editing press enter to upload the updated secret..." key
kubectl create secret generic ${SECRET_NAME} --from-file=remote-bot-config.json --dry-run -o yaml | kubectl apply -f -
cd -
