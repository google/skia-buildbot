cd ~
gclient config https://skia.googlesource.com/buildbot.git --name=skia-buildbot
gclient sync --force
cd skia-buildbot
python scripts/launch_slaves.py