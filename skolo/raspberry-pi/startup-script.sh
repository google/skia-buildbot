#!/bin/sh

# Hack to get swarming access to /b
sudo chmod 777 /b

if [ ! -d "/b/swarm_slave" ]; then
  cd /b
  echo "Bootstrapping swarming, expect a reboot"
  python -c "import urllib; exec urllib.urlopen('https://chromium-swarm.appspot.com/bootstrap').read()"
else
  echo "Starting swarming"
  /usr/bin/python /b/swarm_slave/swarming_bot.zip start_bot &
fi
