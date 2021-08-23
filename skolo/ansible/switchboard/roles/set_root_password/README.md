# Role Name

`set_root_password`

## Description

Sets the root password.

## Variables Required

This role requires the `secrets.skolo_password`, which is loaded via the
required role `load_secret_vars`.

## Example Playbook

```
# Set root password on rpis,
- hosts: rpis
  user: root

  roles:
    - set_root_password
```
