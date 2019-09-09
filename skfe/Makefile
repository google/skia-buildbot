default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "`git log -n1 --format=%s`"

# Build and push targets for nginx-redirect running in skia-public.
k8s_release:
	./build_nginx_redirect_release "`git log -n1 --format=%s`"

k8s_push:
	pushk nginx-redirect

k8s_run:
	# You can test the config locally by running this target and observing that
	# nginx doesn't fail.
	docker run nginx-redirect:latest