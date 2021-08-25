# Role Name

`install_test_machine_monitor`

## Description

Compiles and installs `test_machine_monitor` to the machine and arranges
for it to run at startup. Does *not* restart `test_machine_monitor` if
an old copy is already running; reboot to bring up the new version.

## Variables Required

Requires `gather_facts` to detect the target operating system.

## Optional Variables

The `install_test_machine_monitor__start_swarming` variable controls whether or
not `test_machine_monitor` should be passed the `--start_swarming` flag.

## Example Playbook

```
# Install test_machine_monitor that doesn't launch Swarming.
- hosts: rpis
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_test_machine_monitor

```
