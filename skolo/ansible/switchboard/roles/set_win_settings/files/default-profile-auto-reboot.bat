:: This file exists to work around an issue with auto-login to the chrome-bot account. For unknown
:: reasons, occasionally Windows will have trouble logging in and say "We can't sign in to your
:: account." When this happens, the Default profile is used instead. By placing this file in the
:: Default profile Startup folder, we reboot the machine when this occurs. Normally the next login
:: works fine.

:: Just in case someone creates a new profile and wonders why the machine is rebooting, display a
:: message and give some time before triggering the reboot.
Echo Rebooting due to login with Default profile. Close this window within 10 seconds to cancel.
Timeout /t 10

Echo Rebooting now...
c:\windows\system32\shutdown /r /t 0
