serve-rpi-image/rpi.img:
	gsutil cp gs://skia-backups/rpi_image/2018-08-23-08d6f0b16a83d9961cdd4e79d9baddc7140dca98.gz serve-rpi-image/rpi.img
	# Visually inspect sha1sum of image, should equal the hash in the download name.
	sha1sum serve-rpi-image/rpi.img

build: serve-rpi-image/rpi.img
	docker build -t serve-rpi-image ./serve-rpi-image

VERSION = 2018.08.23.08d6f0.v2
push:
	# format is date of image, first 6 of sha1 of image, then a version (to allow for re-spins)
	docker tag serve-rpi-image gcr.io/skia-public/serve-rpi-image:${VERSION}
	docker push gcr.io/skia-public/serve-rpi-image:${VERSION}
	docker tag serve-rpi-image gcr.io/skia-public/serve-rpi-image:latest
	docker push gcr.io/skia-public/serve-rpi-image:latest
