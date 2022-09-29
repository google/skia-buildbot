# Role Name

`apply_win_os_update`

# Description

Applies Windows 10 feature updates (i.e. OS version updates) via the Windows 10 Update Assistant.

# Example Playbook

```
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - apply_win_os_update
```
