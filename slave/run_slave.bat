set PYTHONPATH=..\third_party\chromium_buildbot\third_party\buildbot_7_12;..\third_party\chromium_buildbot\third_party\twisted_8_1;..\third_party\chromium_buildbot\site_config;..\site_config
python run_slave.py --no_save -y buildbot.tac
