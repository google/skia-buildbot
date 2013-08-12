cd C:\Users\chrome-bot
call gclient config https://skia.googlesource.com/buildbot.git --name=skia-buildbot
call gclient sync --force
cd skia-buildbot
python scripts\launch_slaves.py