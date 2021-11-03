# Role Name

`install_win_gpu_drivers`

# Description

Detects the GPU on a Windows machine and installs the appropriate driver.

# Example Playbook

```
# Installs Graphics Tools on all Windows machines.
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_win_gpu_drivers
```
