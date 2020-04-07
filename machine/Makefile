
default:
	go install -v ./go/...

# Make our own firestore emulator image.
# https://issuetracker.google.com/issues/153087269
release_firestore_emulator:
	./images/firestore-emulator/release

start_firestore_emulator:
	 docker run -ti -p 8011:8011 gcr.io/skia-public/firestore-emulator:2020-04-06T16_43_02Z-jcgregorio-bb5fa1d-dirty gcloud beta emulators firestore start --project testtest --host-port 0.0.0.0:8011