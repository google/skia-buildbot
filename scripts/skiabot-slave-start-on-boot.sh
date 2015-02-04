{
if [ -z "$SKIA_REPO_DIR" ]; then
  SKIA_REPO_DIR="~"
fi
mkdir -p $SKIA_REPO_DIR
cd $SKIA_REPO_DIR
export PATH="$PATH:$HOME/depot_tools"
export DISPLAY=:0
echo "SKIA_REPO_DIR: $SKIA_REPO_DIR"
echo "PATH: $PATH"
gclient config --spec='solutions = [{ "name": "buildbot","url": "https://skia.googlesource.com/buildbot.git","deps_file": "DEPS","managed": True,"custom_deps": {},"safesync_url": "",},{ "name": "src","url": "https://chromium.googlesource.com/chromium/src.git","deps_file": ".DEPS.git","managed": True,"custom_deps": {},"safesync_url": "",},]'
gclient sync --force --verbose
cd buildbot
python scripts/launch_slaves.py
} > ~/reboot.log 2>&1
