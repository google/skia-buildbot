generate_sql_schema:
	cd ./go/schema_generate; go run main.go

build:
	bazelisk build --config=mayberemote -c dbg //comment_rag/...

run-comment-api: build
	../_bazel_bin/comment_rag/go/api/server/server_/server api \
		--config_filename=./configs/demo.json \
		--local
