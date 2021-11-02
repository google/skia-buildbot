# Role Name

`download_win_package_src`

## Description

Downloads the contents of gs://skia-buildbots/skolo/win/win_package_src into a temporary directory
on the local machine, and makes it available to subsequent tasks via the `win_package_src` variable.
The temporary directory will be automatically cleaned up via a handler.

Note that this is a ~10GB download, which can take a few minutes. To skip the download and use a
local copy, invoke your playbook with `--extra-vars win_package_src=path/to/local/copy`.

## Example Playbook

```
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - download_win_package_src

  tasks:
    - name: Copy skolo.pow
      win_copy:
        src: "{{ win_package_src }}/skolo.pow"
        dest: c:\Temp\skolo.pow
```
