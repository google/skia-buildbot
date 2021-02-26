Skia Infra RBE Key Asset
========================

This private asset holds the JSON service account credentials necessary to run Bazel builds against
the RBE instance in the skia-infra-rbe Google Cloud Platform project.

This CIPD package was created by hand with the following invocations:

```
$ mkdir skia_infra_rbe_keys
$ gcloud iam service-accounts keys create ./skia_infra_rbe_key rbe-ci.json \
    --iam-account rbe-ci@skia-public.iam.gserviceaccount.com
$ cipd create -name skia/internal/skia_infra_rbe_key -in ./skia_infra_rbe_key/ -tag version:0
``` 
