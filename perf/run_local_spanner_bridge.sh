#!/bin/bash
# Runs a perf instance against the given spanner database.

# Prerequisite:
# - gcloud auth application-default login
# In Skia Bridge terminal:
# - gcloud config set project chromeperf

# If wanting to edit anomalies, it is important to run breakglass command:
# grants add --wait_for_twosync \
# --reason="b/377751454 -- resolve nudging issue" \
# skia-infra-breakglass-policy:20h

trap terminate SIGINT
terminate() {
  echo
  echo "killing..."
  pkill -SIGINT -P $$

  bridge_pid=$(pgrep -of 'skia_bridge')
  if [ -n "${bridge_pid}" ]; then
    echo "killing bridge ${bridge_pid}"
    kill $bridge_pid
  fi

  perfserver_pid=$(pgrep -of perfserver)
  if [ -n "${perfserver_pid}" ]; then
    echo "killing perfserver ${perfserver_pid}"
    kill $perfserver_pid
  fi

  proxy_pid=$(pgrep -of auth-proxy)
  if [ -n "${proxy_pid}" ]; then
    echo "killing proxy ${proxy_pid}"
    kill $proxy_pid
  fi

  bazel_pid=$(pgrep -of bazel)
  if [ -n "${bazel_pid}" ]; then
    echo "killing bazel ${bazel_pid}"
    kill $bazel_pid
  fi
  echo "killed"
  exit
}

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

# If python env was pass in, then use that, otherwise default.
venv="${HOME}/.venv/bin/activate"
if [[ -n ${env} ]]; then
  venv=$env
fi
echo "Using Python Env Path: ${venv}"

skia_bridge_path="${HOME}/catapult/skia_bridge"
if [[ -n ${bridge} ]]; then
  skia_bridge_path=$bridge
fi
echo "Using Skia Bridge Path: ${skia_bridge_path}"

# Now let's run pgadapter connected to the supplied spanner database.
docker run -d -p 127.0.0.1:5432:5432 \
	-v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
	gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
	-p $p -i $i -d $d -c /acct_credentials.json -x

# Now that pgadapter is connected to the spanner instance, let's run the local frontend
# pointed to pgadapter using the supplied config file.
build_cmd="bazelisk build --config=mayberemote -c dbg //perf/..."
if $( ${build_cmd} ); then
  echo "Build Completed."
else
  echo "Build Failure! Run command manually to debug:"
  echo "  ${build_cmd}"
  terminate
fi

make run-auth-proxy-before-demo-instance  &
source ${venv}
python3 ${skia_bridge_path}/main.py &
deactivate

sleep 10
perf_cmd="../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--localToProd \
		--do_clustering=false \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=${config} \
		--display_group_by=false \
		--disable_git_update=true \
                --disable_metrics_update=true \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:5432/${d}?sslmode=disable"

if $( ${perf_cmd} &); then
  echo "Perfserver Launched..."
else
  echo "Build Failure! Run command manually to debug:"
  echo "  ${perf_cmd}"
  terminate
fi


wait
