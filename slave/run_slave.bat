set THIRDPARTY=..\third_party
set CHROMIUM_BUILDBOT=%THIRDPARTY%\chromium_buildbot
set PUBLIC_CONFIG=%CHROMIUM_BUILDBOT%\site_config
set PRIVATE_CONFIG=..\site_config

set PYTHONPATH=%CHROMIUM_BUILDBOT%\third_party\buildbot_7_12;%CHROMIUM_BUILDBOT%\third_party\twisted_8_1;%PUBLIC_CONFIG%;%PRIVATE_CONFIG%

xcopy /y %PRIVATE_CONFIG%\config_private.py %PUBLIC_CONFIG%
xcopy /y %PRIVATE_CONFIG%\bot_password %PUBLIC_CONFIG%\.bot_password

python run_slave.py --no_save -y buildbot.tac
