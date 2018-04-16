default:
	go install -v ./go/prober ./go/build_probers_json5
	build_probers_json5 --dest=allprobers.json5

validate: default
	prober --config=allprobers.json5 --validate

push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli proberd skia-monitoring

release:
	build_release

push_docker: default
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/prober
	./build_docker_release

push_config: default
	kubectl create configmap prober-config --from-file=allprobers.json5 -o yaml --dry-run | kubectl replace -f -
