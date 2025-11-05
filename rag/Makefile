generate_sql_schema:
	cd ./go/schema_generate; go run main.go

run-auth-proxy:
	bazelisk run //kube/cmd/auth-proxy -- \
		--prom-port=:20003 \
		--role=editor=google.com \
		--authtype=mocked \
		--mock_user=$(USER)@google.com \
		--port=:8003 \
		--passive \
		--target_port=http://127.0.0.1:8002 \
		--local

run-history-api:
	bazelisk run //rag/go/api/server -- \
		api \
		--config_filename=./configs/chrome-internal.json
