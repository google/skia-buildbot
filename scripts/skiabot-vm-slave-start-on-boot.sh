{
while [ -z "$(df -v | grep sdb)" ]; do
  echo "/dev/sdb not yet attached..."
  sleep 1
done
if [ -z "$1" ]; then
  SKIA_REPO_DIR="skia-repo"
else
  SKIA_REPO_DIR="$1"
fi
export PATH="$PATH:/home/default/$SKIA_REPO_DIR/depot_tools"
mkdir -p ~/$SKIA_REPO_DIR
cd ~/$SKIA_REPO_DIR
gclient config gclient config --spec='solutions = [{ "name": "buildbot","url": "https://skia.googlesource.com/buildbot.git","deps_file": "DEPS","managed": True,"custom_deps": {},"safesync_url": "",},{ "name": "src","url": "https://chromium.googlesource.com/chromium/src.git","deps_file": ".DEPS.git","managed": True,"custom_deps": {},"safesync_url": "",},]'
gclient sync --force
cd buildbot
python scripts/launch_slaves.py
} > ~/reboot.log 2>&1
