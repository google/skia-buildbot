#/bin/bash

# Downloads the secret login.json file for editing, i.e. to change the salt,
# client_id or client_secret.
#
# Once you are done editing the file, the config will be uploaded.
SECRET_NAME=swarming-netrc
source ../bash/ramdisk.sh
source ../kube/clusters.sh

__skia_rpi2

cd /tmp/ramdisk

kubectl get secret ${SECRET_NAME} -o json | jq -r '.data[".netrc"]' | base64 -d > .netrc

echo "Downloaded the login secrets to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/.netrc"
echo ""
read -r -p "When you are done editing press enter to upload the updated login.json file..." key
kubectl create secret generic ${SECRET_NAME} --from-file=.netrc --dry-run -o yaml | kubectl apply -f -
cd -
