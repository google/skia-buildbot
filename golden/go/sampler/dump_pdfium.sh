#!/bin/bash

set -x -e

go run main.go --logtostderr \
               --trace_service=:10001 \
               --git_repo_dir=./pdfium \
               --git_repo_url=https://pdfium.googlesource.com/pdfium \
               --db_host localhost \
               --db_name pdfium_gold_skiacorrectness \
               --db_user readwrite \
               --n_commits 100 \
               --output_file pdfium.tile
