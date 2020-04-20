#!/bin/bash
#
# Setup the local machine for running cluster telemetry master scripts with --local=true.
# This script assumes you are a Googler running Goobuntu.
#
# TODO(benjaminwagner): Google Cloud SDK?

if [[ -z "$GOPATH" ]]; then
  echo "Please set GOPATH environment variable." 1>&2
  exit 1
fi

set -x
set -e

echo "Installing debs..."
sudo apt-get update
sudo apt-get -y install linux-tools-generic python-django libgif-dev && sudo easy_install -U pip && sudo pip install setuptools --no-use-wheel --upgrade && sudo pip install -U crcmod

echo "Creating /usr/local/google/cluster-telemetry/b and linking to /b..."
mkdir -p /usr/local/google/cluster-telemetry/b
sudo ln -s /usr/local/google/cluster-telemetry/b /

echo "Downloading depot_tools..."
cd /b/
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git

echo "Checking out Skia's buildbot and trunk..."
mkdir -p /b/storage/glog
mkdir /b/skia-repo/
cd /b/skia-repo/
/b/depot_tools/gclient config https://skia.googlesource.com/buildbot.git
/b/depot_tools/gclient sync
sed -i '$ d' .gclient && sed -i '$ d' .gclient
echo """
  { 'name'        : 'trunk',
    'url'         : 'https://skia.googlesource.com/skia.git',
    'deps_file'   : 'DEPS',
    'managed'     : True,
    'custom_deps' : {
    },
    'safesync_url': '',
  },
]
""" >> .gclient
/b/depot_tools/gclient sync

echo "Linking infra repo at $GOPATH to /b/skia-repo/go..."
ln -s $GOPATH /b/skia-repo/go

echo "Creating test webhook_salt.data..."
echo -n "notverysecret" | base64 -w 0 > /b/storage/webhook_salt.data

echo "Checking out Chromium..."
mkdir -p /b/storage/chromium
cd /b/storage/chromium
/b/depot_tools/fetch --nohooks --no-history chromium
cd src
echo "N" | ./build/install-build-deps.sh
/b/depot_tools/gclient runhooks

rsa_key="${HOME}/.ssh/id_rsa"
if [[ ! -e "${rsa_key}" ]]; then
  echo "Setting up passwordless SSH to the local machine..."
  ssh-keygen -t rsa -N '' -f "${rsa_key}"
  cat "${rsa_key}.pub" >> "${HOME}/.ssh/authorized_keys"
  set +x
  echo
  echo "Warning: Using ${rsa_key} for remote access is not advisable. (I.e. do not add ${rsa_key}.pub to any remote machine authorized_keys.)"
  echo "For remote SSH access, follow instructions at http://go/gnubbyssh"
elif grep -L ENCRYPTED "${rsa_key}"; then
  set +x
  echo
  echo "${rsa_key} has a passphrase. Master scripts will not be able to run worker scripts on this machine."
fi

set +x

echo
echo "The setup script has completed! Manual steps required:"
echo
echo " - Go to https://console.cloud.google.com/apis/credentials/oauthclient/31977622648-ubjke2f3staq6ouas64r31h8f8tcbiqp.apps.googleusercontent.com?project=google.com:skia-buildbots, click Download JSON, and save it to /b/storage/client_secret.json."
echo " - Run a master script from the command line to set up Gmail and Google Storage authorization tokens."
echo "     Here is an example of running a master script:"
echo '     $ cd /b/skia-repo/go/src/go.skia.org/infra/ct'
echo '     $ make all'
echo '     $ RUN_ID="setup_local-$(date '"'"'+%s'"'"')"'
echo '     $ run_chromium_perf_on_workers --emails=$USER@google.com --gae_task_id=0 --run_id=${RUN_ID} --pageset_type=Dummy1k --benchmark_name=rasterize_and_record_micro --log_id=${RUN_ID} --local=true --alsologtostderr'
echo "     (run_chromium_perf_on_workers will fail when it can't find a patch, but as long as it fails after requesting a Gmail auth token and Google Storage auth token, it's ok.)"

