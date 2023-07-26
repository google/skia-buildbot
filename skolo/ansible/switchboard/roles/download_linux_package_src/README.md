# Role Name

`download_linux_packages_src`

## Description

This role is the Linux equivalent of the `download_win_packages_src` role.

Downloads the contents of `gs://skia-buildbots/skolo/linux/linux_package_src` into a temporary
directory on the local machine, and makes it available to subsequent tasks via the
`linux_package_src` variable. The temporary directory will be automatically cleaned up via a
handler.

If the GCS directory grows too big, this might take several minutes. To skip the download and use a
local copy, invoke your playbook with `--extra-vars linux_package_src=path/to/local/copy`.

## Example Playbook

```yml
- hosts: all_linux
  user: chrome-bot
  gather_facts: yes

  roles:
    - download_linux_packages_src

  tasks:
    - name: Copy Nvidia driver installer
      copy:
        src: '{{ linux_package_src }}/NVIDIA-Linux-x86_64-535.86.05.run'
        dest: /tmp/NVIDIA-Linux-x86_64-535.86.05.run
```
