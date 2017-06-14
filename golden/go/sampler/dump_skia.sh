#!/bin/bash

set -e -x

go run main.go --logtostderr \
               --trace_service=:9001 \
               --git_repo_dir=./skia \
               --git_repo_url=https://skia.googlesource.com/skia \
               --db_host localhost \
               --db_name gold_skiacorrectness \
               --db_user readwrite \
               --n_commits 50 \
               --output_file skia.tile
