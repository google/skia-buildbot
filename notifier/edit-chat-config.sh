#/bin/bash

# Downloads the secret chat_config.txt file for editing, i.e. to add, remove,
# or update new chatroom targets.
#
# Once you are done editing the file, the config will be uploaded.
source ../bash/ramdisk.sh
cd /tmp/ramdisk
kubectl get secret notifier-chat-config -o json | jq -r '.data["chat_config.txt"]' | base64 -d > chat_config.txt

echo "Downloaded the chat config to /tmp/ramdisk."
echo "Edit /tmp/ramdisk/chat_config.txt."
echo ""
read -r -p "When you are done editing press enter to upload the updated chat_config.txt file..." key
kubectl create secret generic notifier-chat-config --from-file=chat_config.txt=chat_config.txt --dry-run -o yaml | kubectl apply -f -
cd -
