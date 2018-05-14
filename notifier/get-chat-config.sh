#/bin/bash

# Downloads the secret chat_config.txt file for editing, i.e. to add, remove,
# or update new chatroom targets.
#
# Once you are done editing the file, upload it with 'put-chat-config.sh'.
kubectl get secret alertmanager-webhook-chat-config -o json | jq -r '.data["chat_config.txt"]' | base64 -d > chat_config.txt
