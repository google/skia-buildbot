cd C:\Users\chrome-bot
call gclient config https://skia.googlesource.com/buildbot.git
call gclient sync --force
cd buildbot
python scripts\launch_slaves.py