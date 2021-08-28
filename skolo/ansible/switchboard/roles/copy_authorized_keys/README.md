# Role Name

`copy_authorized_keys`

# Description

Distributes the file `//skolo/authorized_keys` to the correct account on the
machine.

Platforms specific tasks are in their own files, e.g. `linux.yml`.

# Variables Required

This role uses the `skolo_account` variable defined in `hosts.yml`.

Also requires `gather_facts` to detect the target operating system.

# Example Playbook

```
# Copy the authorized_keys files to all the RPis.
- hosts: rpis
  user: chrome-bot
  gather_facts: yes

  roles:
    - copy_authorized_keys
```
