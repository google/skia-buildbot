#/bin/bash

# Downloads the secret grafana.ini file for editing.
#
# Once you are done editing the file, the config will be uploaded.
SECRET_NAME=grafana-ini
source ../kube/config.sh
source ../bash/ramdisk.sh
cd /tmp/ramdisk
kubectl get secret ${SECRET_NAME} -o json | jq -r '.data["grafana.ini"]' | base64 -d > grafana.ini

echo "Downloaded the login secrets to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/grafana.ini."
echo ""
read -r -p "When you are done editing press enter to upload the updated grafana.ini file..." key
kubectl create secret generic ${SECRET_NAME} --from-file=grafana.ini --dry-run=client -o yaml | kubectl apply -f -
cd -
echo "Now restart the grafana-0 pod so it picks up the config change."
