#!/bin/bash

mkdir -p ./logs
GOOGLE_APPLICATION_CREDENTIALS=$HOME/service-accounts/machine-manager-testing.json \
go run ./cmd/machine_manager --logtostderr 2> ./logs/machine_manager.log &
GOOGLE_APPLICATION_CREDENTIALS=$HOME/service-accounts/machine-manager-testing.json \
go run ./cmd/emulated_task_scheduler --logtostderr 2> ./logs/task_scheduler.log &

GOOGLE_APPLICATION_CREDENTIALS=$HOME/service-accounts/machine-daemon-testing.json \
go run ./cmd/emulated_machine --logtostderr

sleep 1
killall machine_manager
killall -r .*emulated_task.*