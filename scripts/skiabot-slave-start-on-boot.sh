{
cd ~
export PATH="$PATH:$(pwd)/depot_tools"
gclient config https://skia.googlesource.com/buildbot.git
gclient sync --force
cd buildbot
python scripts/launch_slaves.py
} > ~/reboot.log 2>&1
