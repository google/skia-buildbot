cd ~
gclient config https://skia.googlesource.com/buildbot.git
gclient sync --force
cd buildbot
python scripts/launch_slaves.py