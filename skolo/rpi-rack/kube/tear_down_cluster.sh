#!/bin/bash

set -e -x

kubectl drain $1 --delete-local-data --force --ignore-daemonsets
kubectl delete node $1
sudo kubeadm reset -f
rm -rf ${HOME}/.kube
