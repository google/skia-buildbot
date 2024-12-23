#!/bin/bash
# Runs a perf instance against the given spanner database.

# First delete any existing docker containers to start clean.
sudo docker ps -q | xargs -r sudo docker rm -vf

# Now let's get all the arguments passed in.
for arg in "$@"
do
    argKey=$(echo $arg | cut -f1 -d=)
    keyLen=${#argKey}
    val="${arg:$keyLen+1}"

    export "$argKey"="$val"
done

echo "Using the following params: -p=$p -i=$i -d=$d -config=$config"

# Now let's run pgadapter connected to the supplied spanner database.
docker run -d -p 127.0.0.1:5432:5432 \
	-v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
	gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
	-p $p -i $i -d $d -c /acct_credentials.json -x

# Now that pgadapter is connected to the spanner instance, let's run the local frontend
# pointed to pgadapter using the supplied config file.
bazelisk build --config=mayberemote -c dbg //perf/...
../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--do_clustering=false \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=$config \
		--display_group_by=false \
		--disable_git_update=true \
        --disable_metrics_update=true \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:5432/chrome_pub?sslmode=disable
