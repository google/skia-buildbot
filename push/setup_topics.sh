#/bin/bash
#
# Sets up all the access rights that the skia-push service account needs.
set -x

# Pub Sub
## Topics
gcloud pubsub topics create push_command
gcloud pubsub topics create push_status

gcloud beta pubsub topics \
  add-iam-policy-binding \
  projects/google.com:skia-buildbots/topics/push_command \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com \
  --role roles/pubsub.publisher

gcloud beta pubsub topics \
  add-iam-policy-binding \
  projects/google.com:skia-buildbots/topics/push_status \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  \
  --role roles/pubsub.publisher

gcloud beta pubsub topics \
  add-iam-policy-binding \
  projects/google.com:skia-buildbots/topics/push_command \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com \
  --role roles/pubsub.subscriber

gcloud beta pubsub topics \
  add-iam-policy-binding \
  projects/google.com:skia-buildbots/topics/push_status \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  \
  --role roles/pubsub.subscriber

## Subscriptions
gcloud beta pubsub subscriptions create \
  projects/google.com:skia-buildbots/subscriptions/pulld \
  --topic projects/google.com:skia-buildbots/topics/push_command \
  --topic-project google.com:skia-buildbots \
  --ack-deadline 10

gcloud beta pubsub subscriptions create \
  projects/google.com:skia-buildbots/subscriptions/push \
  --topic projects/google.com:skia-buildbots/topics/push_status \
  --topic-project google.com:skia-buildbots \
  --ack-deadline 10

gcloud beta pubsub subscriptions add-iam-policy-binding \
  projects/google.com:skia-buildbots/subscriptions/push \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  \
  --role roles/pubsub.admin

gcloud beta pubsub subscriptions add-iam-policy-binding \
  projects/google.com:skia-buildbots/subscriptions/pulld \
  --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  \
  --role roles/pubsub.admin

# GCS Bucket
gsutil iam ch \
  serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com:objectAdmin \
  gs://skia-push
