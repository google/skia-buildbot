#/bin/bash

# Downloads the adbkey and adbkey.pus secret files for editing.
#
# Once you are done editing the file, the config will be uploaded.
SECRET_NAME=skolo-rpi-adb-key
source ../bash/ramdisk.sh

CLUSTER=$(kubectl config current-context)
if [ "$CLUSTER" != "skolo_rpi2" ]
then
  echo "Wrong cluster, must be run in skolo_rpi2."
  exit 1
fi

cd /tmp/ramdisk

kubectl get secret ${SECRET_NAME} -o json | jq -r '.data["adbkey"]' | base64 -d > adbkey
kubectl get secret ${SECRET_NAME} -o json | jq -r '.data["adbkey.pub"]' | base64 -d > adbkey.pub

echo "Downloaded the adbkey secrets to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/abdkey and /tmp/ramdisk/adbkey.pub"
echo ""
read -r -p "When you are done editing press enter to upload the updated login.json file..." key
kubectl create secret generic ${SECRET_NAME} --from-file=. --dry-run -o yaml | kubectl apply -f -
cd -
