include ../make/bazel.mk

build:
	${BAZEL} build go/...

release: build
	${BAZEL} run //switchboard:push_switchboard_container

push: release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout switch-pod

test:
	go test ./go/...

run-local-instance: build
	../_bazel_bin/switchboard/go/switch-pod-monitor/switch-pod-monitor_/switch-pod-monitor --local
