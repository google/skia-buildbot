#/bin/bash

gcloud beta container --project "skia-public" clusters create "skia-public" \
--zone "us-central1-a" \
--no-enable-basic-auth \
--cluster-version "1.8.8-gke.0" \
--machine-type "n1-standard-8" \
--image-type "COS" \
--disk-size "100" \
--num-nodes "3" \
--network "default" \
--enable-cloud-logging \
--enable-cloud-monitoring \
--subnetwork "default" \
--enable-autoscaling --min-nodes "3" --max-nodes "30" \
--enable-autoupgrade
