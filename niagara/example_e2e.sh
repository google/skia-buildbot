#!/bin/bash

go run ./cmd/machine_manager --logtostderr 2> ./logs/machine_manager.log &
go run ./cmd/emulated_task_scheduler --logtostderr 2> ./logs/task_scheduler.log &
go run ./cmd/emulated_machine --logtostderr

sleep 1
killall machine_manager
killall -r .*emulated_task.*