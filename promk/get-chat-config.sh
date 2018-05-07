#/bin/bash
kubectl get secret  alertmanager-webhook-chat-config -o json | jq -r '.data["chat_config.txt"]' | base64 -d > chat_config.txt
