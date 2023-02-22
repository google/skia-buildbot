# Role Name

`skia_needs`

# Description

This role installs any dependencies needed to compile and test Skia.

# Example Playbook

```
- hosts: gce_linux
  user: chrome-bot

  roles:
    - skia_needs
```
