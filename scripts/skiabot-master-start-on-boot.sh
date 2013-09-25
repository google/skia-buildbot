if [ -z "$1" ]; then
  SKIA_REPO_DIR="skia-master"
else
  SKIA_REPO_DIR="$1"
fi
mkdir -p ~/$SKIA_REPO_DIR
cd ~/$SKIA_REPO_DIR
gclient config https://skia.googlesource.com/buildbot.git
gclient sync --force
cd buildbot
python scripts/launch_master.py --loop

