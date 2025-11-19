#!/bin/bash
# Runs a perf instance against the given spanner database.
# Prerequisite:
# gcloud auth application-default login

# Example Command:
# ./run_with_spanner.sh p=skia-infra-corp i=tfgen-spanid-20241205020733610
# d=v8_int config=configs/spanner/v8-internal.json repo=v8/v8

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

# Check if domain or repo are set via params, if not set them to default values.
if [[ -z "${domain}" ]]; then
  domain="chromium.googlesource.com"
fi

if [[ -z "${repo}" ]]; then
  repo="chromium/src"
fi

echo "Using the following params: -p=$p -i=$i -d=$d -config=$config -domain=$domain -repo=$repo"
# Now let's run pgadapter connected to the supplied spanner database.
sudo docker run -d -p 127.0.0.1:5432:5432 \
  -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
  gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
  -p $p -i $i -d $d -c /acct_credentials.json -x
# Now that pgadapter is connected to the spanner instance, let's run the local frontend
# pointed to pgadapter using the supplied config file. First, build the perfserver.
bazelisk build --config=mayberemote -c dbg //perf/go/perfserver //perf/pages/... || {
  echo "Build failed, exiting (not trying to run outdated perfserver)."
  exit 1
}
../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
  --dev_mode \
  --localToProd \
  --do_clustering=false \
  --port=:8002 \
  --prom_port=:20001 \
  --config_filename=$config \
  --display_group_by=false \
  --disable_metrics_update=true \
  --resources_dir=../_bazel_bin/perf/pages/development/ \
  --connection_string=postgresql://root@127.0.0.1:5432/${d}?sslmode=disable \
  --commit_range_url=https://${domain}/${repo}/+log/{begin}..{end}
