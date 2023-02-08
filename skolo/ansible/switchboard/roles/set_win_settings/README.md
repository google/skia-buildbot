# Role Name

`set_win_settings`

## Description

Sets a Windows machine's various settings.

### Common tasks

Disables disk indexing. Sets up auto logon for the chrome-bot user.

### Skolo-specific tasks

Sets a Windows machine's time zone and power settings. Disables Windows Defender, which cannot be
uninstalled on Windows 10.

For unknown reasons, occasionally Windows will have trouble logging in and say "We can't sign in
to your account." When this happens, the Default profile is used instead. As a workaround, this role
adds a script to the Default profile's Startup folder that reboots the machine when the Default
profile is used.

### GCE-specific tasks

Uninstalls Windows Defender. This is possible because GCE Windows machines run Windows Server.

## Example Playbook

```
- hosts: all_win
  user: chrome-bot

  roles:
    - set_win_settings
```
