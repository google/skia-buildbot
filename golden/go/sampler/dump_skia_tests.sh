#!/bin/bash

set -e -x

# query=name%3Darithmode%26name%3Dblurrects%26name%3Dconvex-lineonly-paths%26name%3Dgradients_interesting%26name%3Dimagefilterscropexpand%26name%3Drepeated_bitmap_jpg%26source_type%3Dgm
# query=name=arithmode&name=blurrects&name=convex-lineonly-paths&name=gradients_interesting&name=imagefilterscropexpand&name=repeated_bitmap_jpg&source_type=gm

go run main.go --logtostderr \
               --trace_service=:9001 \
               --git_repo_dir=./skia \
               --git_repo_url=https://skia.googlesource.com/skia \
               --db_host localhost \
               --db_name gold_skiacorrectness \
               --db_user readwrite \
               --n_commits 50 \
               --output_file skia.tile \
               --query "name=arithmode&name=blurrects&name=convex-lineonly-paths&name=gradients_interesting&name=imagefilterscropexpand&name=repeated_bitmap_jpg&source_type=gm"
