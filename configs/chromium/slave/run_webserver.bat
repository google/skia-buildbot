@echo off
set lighttpd_dir=%~dp0..\third_party\lighttpd\win

REM copy lighttpd.exe to lighttpd_server.exe, because we don't want it to be
REM killed by taskkill.

xcopy /D %lighttpd_dir%\lighttpd.exe %lighttpd_dir%\lighttpd_server.exe

REM copy the cygwin dll to the lighttpd folder because otherwise it wont work.

xcopy /D %lighttpd_dir%\..\no_dll\CygWin1.dll %lighttpd_dir%

REM Start the server, using the conf file in the slave directory.

@echo on
%lighttpd_dir%\lighttpd_server.exe -f %~dp0\lighttpd.conf -m %lighttpd_dir%\lib