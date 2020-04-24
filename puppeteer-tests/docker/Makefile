.PHONY: push
push: container
	docker push gcr.io/skia-public/puppeteer-tests:latest

.PHONY: container
container:
	docker build -t puppeteer-tests -f Dockerfile .
	docker tag puppeteer-tests gcr.io/skia-public/puppeteer-tests:latest
