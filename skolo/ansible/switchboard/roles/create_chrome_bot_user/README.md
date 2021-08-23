# Role Name

`create_chrome_bot_user`

## Description

Creates the 'chrome-bot' user.

## Variables Required

This role requires the `secrets.skolo_password`, which is loaded via the
required role `load_secret_vars`.

Also requires `gather_facts` to detect the target operating system.

## Example Playbook

```
# Create the chrome-bit user on all the RPis.
- hosts: rpis
  user: chrome-bot
  gather_facts: yes

  roles:
    - create_chrome_bot_user
```
