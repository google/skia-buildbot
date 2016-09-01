include ../go/skiaversion/skiaversion.mk

all: task_scheduler

task_scheduler: skiaversion
	go install -v ./go/task_scheduler

testgo: skiaversion
	go test ./go/... -v
