default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "`git log -n1 --format=%s`"

# Build and push targets for nginx-skia-org running in skia-public.
k8s_release:
	./build_services_conf
	./build_nginx_skia_org_release "`git log -n1 --format=%s`"

k8s_push: k8s_release
	pushk nginx-skia-org

k8s_run:
	# You can test the config locally by running this target and observing that
	# nginx doesn't fail.
	docker run nginx-skia-org:latest