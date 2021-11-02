# Role Name

`set_win_skolo_settings`

## Description

Sets a Windows machine's time zone and power settings. Disables Windows Defender, and disk indexing.

For unknown reasons, occasionally Windows will have trouble logging in and say "We can't sign in
to your account." When this happens, the Default profile is used instead. As a workaround, this role
adds a script to the Default profile's Startup folder that reboots the machine when the Default
profile is used.

## Example Playbook

```
- hosts: all_win
  user: chrome-bot

  roles:
    - set_win_skolo_settings
```
