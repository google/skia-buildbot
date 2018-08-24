IMG_SHA1 = 08d6f0b16a83d9961cdd4e79d9baddc7140dca98

serve-rpi-image/rpi.img:
	gsutil cp gs://skia-backups/rpi_image/2018-08-23-${IMG_SHA1}.gz serve-rpi-image/rpi.img
	# Check that the sha1sum matches using the --check mode of sha1sum.
	# The way the files are stored in skia-backups means the hash in the name
	# should match the hash of the unzipped image.
	bash -c "echo '$(IMG_SHA1) serve-rpi-image/rpi.img' | sha1sum -c -"

.PHONY: build
build: serve-rpi-image/rpi.img
	docker build -t serve-rpi-image ./serve-rpi-image

VERSION = 2018.08.23.08d6f0.v2

.PHONY: push
push:
	# format is date of image, first 6 of sha1 of image, then a version (to allow for re-spins)
	docker tag serve-rpi-image gcr.io/skia-public/serve-rpi-image:${VERSION}
	docker push gcr.io/skia-public/serve-rpi-image:${VERSION}
	docker tag serve-rpi-image gcr.io/skia-public/serve-rpi-image:latest
	docker push gcr.io/skia-public/serve-rpi-image:latest
