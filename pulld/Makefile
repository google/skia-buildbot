default:
	go install -v ./go/pulld
	npx webpack

prod:
	go install -v ./go/pulld
	NODE_ENV=production npx webpack

serve:
	npx webpack-dev-server --content-base ./dist --watch

watch:
	npx webpack --watch

release: prod
	./build_release "$(MESSAGE)"

release_not_gce: default
	go install -v ../get_service_account/go/get_service_account
	get_service_account
	./build_release_not_gce "$(MESSAGE)"

release_arm: default
	go install -v ../get_service_account/go/get_service_account
	get_service_account
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/pulld_arm -v ./go/pulld
	./build_release_arm "$(MESSAGE)"

# You should run make release first
push_everywhere:
	cd ../push; make build
	pushcli --alsologtostderr --config_filename=../push/allskiapush.json5 pulld "*"
