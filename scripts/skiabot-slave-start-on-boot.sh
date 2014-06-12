{
cd ~
export PATH="$PATH:$(pwd)/depot_tools"
gclient config gclient config --spec='solutions = [{ "name": "buildbot","url": "https://skia.googlesource.com/buildbot.git","deps_file": "DEPS","managed": True,"custom_deps": {},"safesync_url": "",},{ "name": "src","url": "https://chromium.googlesource.com/chromium/src.git","deps_file": ".DEPS.git","managed": True,"custom_deps": {},"safesync_url": "",},]'
gclient sync --force
cd buildbot
python scripts/launch_slaves.py
} > ~/reboot.log 2>&1
