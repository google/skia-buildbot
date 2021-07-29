# Role Name

`swarming_needs`

# Description

All the extra things that need to be on a test machine for Swarming to run.

Right now this only works for linux machines, it will need to evolve to support
all the platforms, at which point the platforms specific tasks should go in
their own files, e.g. `linux.yml`.

# Variables Required

None.

# Example Playbook

```
# Copy the authorized_keys files to all the RPis.
- hosts: "{{ variable_hosts | default('rpis') }}"
  user: chrome-bot

  roles:
    - swarming_needs
```
