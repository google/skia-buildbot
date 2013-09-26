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
gclient config https://skia.googlesource.com/buildbot.git
gclient sync --force
cd buildbot
python scripts/launch_slaves.py
} > ~/reboot.log 2>&1
