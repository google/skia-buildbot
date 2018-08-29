.PHONY: skia-public skia-corp

skia-public:
	gcloud config set project skia-public
	gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public

skia-corp:
	gcloud config set project google.com:skia-corp
	gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp
