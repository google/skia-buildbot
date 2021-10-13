# Role Name

`install_win_graphics_tools`

# Description

Installs
[Graphics Tools for Windows 10](https://docs.microsoft.com/en-us/visualstudio/debugger/graphics/getting-started-with-visual-studio-graphics-diagnostics)
on a Windows machine.

# Example Playbook

```
# Installs Graphics Tools on all Windows machines.
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_win_graphics_tools
```
