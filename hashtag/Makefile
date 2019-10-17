default: build package-lock.json
	npx webpack --mode=development

build:
	go install ./go/hashtag

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/hashtag
	./build_hashtag_release

push: release
	pushk --cluster=skia-public hashtag

watch: package-lock.json
	npx webpack --mode=development --watch

test:
	go test ./go/...

package-lock.json: package.json
	npm install