# Role Name

`install_test_machine_monitor`

## Description

Compiles and installs `test_machine_monitor` to the machine.

## Variables Required

Requires `gather_facts` to detect the target operating system.

This role uses the `skolo_account` variable defined in
`//skolo/ansible/group_vars/all.yml` and potentially overridden in `hosts.ini`.

## Arguments

Takes a single boolean argument `start_swarming` that controls whether or not
`test_machine_monitor` should be passed the `--start_swarming` flag.

## Example Playbook

```
# Install test_machine_monitor that doesn't launch Swarming.
- hosts: "{{ variable_hosts | default('rpis') }}"
  user: chrome-bot
  gather_facts: yes

  roles:
    - { role: install_test_machine_monitor, start_swarming: false }
```
