.PHONY: push
push: container
	docker push gcr.io/skia-public/gold-puppeteer-tests:latest

.PHONY: container
container:
	docker build -t gold-puppeteer-tests -f Dockerfile .
	docker tag gold-puppeteer-tests gcr.io/skia-public/gold-puppeteer-tests:latest
