#/bin/bash
#
# Sets up all the access rights that the skia-push service account needs.
set -x
gcloud pubsub topics create push_command
gcloud pubsub topics create push_status
gcloud beta pubsub topics add-iam-policy-binding projects/google.com:skia-buildbots/topics/push_command --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  --role=roles/pubsub.publisher
gcloud beta pubsub topics add-iam-policy-binding projects/google.com:skia-buildbots/topics/push_command --member serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com  --role=roles/pubsub.subscriber
gsutil iam ch serviceAccount:skia-push@skia-buildbots.google.com.iam.gserviceaccount.com:objectAdmin
