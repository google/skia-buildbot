#/bin/bash

accounts=(
afdo-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
angle-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
angle-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-buildbots.google.com@appspot.gserviceaccount.com
catapult-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
chrome-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com
chromite-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
chromium-swarm-bots@skia-buildbots.google.com.iam.gserviceaccount.com
ct-internal-swarming@skia-buildbots.google.com.iam.gserviceaccount.com
ct-swarming@skia-buildbots.google.com.iam.gserviceaccount.com
depot-tools-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
fuchsia-sdk-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
gold-ingestion@skia-buildbots.google.com.iam.gserviceaccount.com
internal-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com
ios-internal-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
jumphost-service@skia-buildbots.google.com.iam.gserviceaccount.com
nacl-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
pdfium-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skcms-skia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-flutter-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-fuchsia-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-internal-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
skia-internal-compile-tasks@skia-buildbots.google.com.iam.gserviceaccount.com
skia-internal-gm-uploader@skia-buildbots.google.com.iam.gserviceaccount.com
skia-internal-nano-uploader@skia-buildbots.google.com.iam.gserviceaccount.com
skia-internal-tasks@skia-buildbots.google.com.iam.gserviceaccount.com
skia-push@skia-buildbots.google.com.iam.gserviceaccount.com
src-internal-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com
status@skia-buildbots.google.com.iam.gserviceaccount.com
status-internal@skia-buildbots.google.com.iam.gserviceaccount.com
task-scheduler@skia-buildbots.google.com.iam.gserviceaccount.com
task-scheduler-internal@skia-buildbots.google.com.iam.gserviceaccount.com
webrtc-chromium-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com)

for account in "${accounts[@]}"
do
  gcloud beta projects add-iam-policy-binding google.com:skia-buildbots --member serviceAccount:$account --role projects/google.com:skia-buildbots/roles/base_image
  gcloud beta projects add-iam-policy-binding google.com:skia-buildbots --member serviceAccount:$account --role projects/google.com:skia-buildbots/roles/pulld_participant
done
