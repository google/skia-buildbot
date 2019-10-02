#!/bin/bash

# Kill services and ConfigMaps on skia-corp.
gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp
kubectl delete deployment gold-goldpushk-corp-test1-healthy-server \
                          gold-goldpushk-corp-test1-crashing-server \
                          gold-goldpushk-corp-test2-healthy-server \
                          gold-goldpushk-corp-test2-crashing-server
kubectl delete configmap  gold-goldpushk-corp-test1-healthy-server-config \
                          gold-goldpushk-corp-test2-healthy-server-config

# Kill services and ConfigMaps on skia-public.
gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public
kubectl delete deployment gold-goldpushk-test1-healthy-server \
                          gold-goldpushk-test1-crashing-server \
                          gold-goldpushk-test2-healthy-server \
                          gold-goldpushk-test2-crashing-server
kubectl delete configmap  gold-goldpushk-test1-healthy-server-config \
                          gold-goldpushk-test2-healthy-server-config
