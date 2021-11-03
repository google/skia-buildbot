# Role Name

`install_vnc_server`

# Description

Installs and sets up a VNC server.

# Example Playbook

```
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_vnc_server
```
