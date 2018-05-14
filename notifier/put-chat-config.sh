#/bin/bash

# Uploads the chat_config.txt configuration file for outgoing chat messages.
# Stored as a secret since the URLs are webhooks with their secrets in the
# URL. Deletes the file after upload.
#
# See the get-chat-config.sh for downloading the chat_config.txt file for
# editing.
kubectl create secret generic notifier-chat-config --from-file=chat_config.txt=chat_config.txt --dry-run -o yaml | kubectl apply -f -
rm chat_config.txt
