#/bin/bash
kubectl create secret generic alertmanager-webhook-chat-config --from-file=chat_config.txt=chat_config.txt --dry-run -o yaml | kubectl apply -f -
rm chat_config.txt
