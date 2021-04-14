default: npm-ci
	go install -v ./go/pulld
	npx webpack --mode=development

prod: npm-ci
	go install -v ./go/pulld
	npx webpack --mode=production

serve: npm-ci
	npx webpack-dev-server --content-base ./dist --watch --mode=development

watch: npm-ci
	npx webpack --watch --mode=development

release: prod
	./build_release "$(MESSAGE)"

release_not_gce: default
	./build_release_not_gce "$(MESSAGE)"

release_arm: default
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/pulld_arm -v ./go/pulld
	./build_release_arm "$(MESSAGE)"

# You should run make release first
push_everywhere:
	cd ../push; make build
	pushcli --alsologtostderr --config_filename=../push/allskiapush.json5 pulld "*"

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

include ../make/npm.mk
