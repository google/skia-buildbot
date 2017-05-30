include ../go/skiaversion/skiaversion.mk

.PHONY: all
all: remote-control

.PHONY: remote-control
remote-control: skiaversion
	go install -v ./go/remote-control

.PHONY: test
test: remote-control
	go test ./go/... -v --small --medium --large

.PHONY: push
push: all
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli remote-control skia-remote-control

.PHONY: vm_create_prod
vm_create_prod:
	go run ./vm.go --alsologtostderr --create --ignore-exists --instance=prod --workdir ../

.PHONY: vm_delete_prod
vm_delete_prod:
	go run ./vm.go --alsologtostderr --delete --ignore-exists --instance=prod --workdir ../

.PHONY: vm_create_test
vm_create_test:
	go run ./vm.go --alsologtostderr --create --ignore-exists --instance=test --workdir ../

.PHONY: vm_delete_test
vm_delete_test:
	go run ./vm.go --alsologtostderr --delete --ignore-exists --instance=test --delete-data-disk --workdir ../

