@echo off
setlocal
title Build slave
if not exist %~dp0..\..\depot_tools\. (
  echo You must put a copy of depot_tools in %~dp0..\depot_tools
  echo Did you read the instructions carefully??
  goto :EOF
)
set PATH=%~dp0..\..\depot_tools;%PATH%
set PYTHONPATH=%~dp0..\third_party\buildbot_7_12;%~dp0..\third_party\twisted_8_1
:: Running it once will make sure svn and python were downloaded.
call gclient.bat > nul
:: run_slave.py will overwrite the PATH and PYTHONPATH environment variables.
python %~dp0\run_slave.py --no_save -y buildbot.tac
