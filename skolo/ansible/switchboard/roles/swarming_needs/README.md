# Role Name

`swarming_needs`

# Description

All the extra things that need to be on a test machine for Swarming to run.

Right now this only works for linux machines, it will need to evolve to support
all the platforms, at which point the platforms specific tasks should go in
their own files, e.g. `linux.yml`.

# Variables

`swarming_needs.needs_mobile_tools` controls the installation of `adb` and the
`idevice*` applications.

# Example Playbook

```
# Copy the authorized_keys files to all the RPis.
- hosts: rpis
  user: chrome-bot

  roles:
    - swarming_needs
```
