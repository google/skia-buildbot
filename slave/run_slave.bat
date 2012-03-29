set THIRDPARTY=..\third_party
set CHROMIUM_BUILDBOT=%THIRDPARTY%\chromium_buildbot
set PUBLICCONFIG_DIR=%CHROMIUM_BUILDBOT%\site_config
set PRIVATECONFIG_DIR=..\site_config

set PYTHONPATH=%CHROMIUM_BUILDBOT%\third_party\buildbot_7_12;%CHROMIUM_BUILDBOT%\third_party\twisted_8_1;%PUBLICCONFIG_DIR%;%PRIVATECONFIG_DIR%

echo fake bot_password to quiet run_slave.py >%PUBLICCONFIG_DIR%\.bot_password
python %CHROMIUM_BUILDBOT%\slave\run_slave.py --no_save -y buildbot.tac
