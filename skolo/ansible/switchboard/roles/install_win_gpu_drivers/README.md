# Role Name

`install_win_gpu_drivers`

# Description

Detects the GPU on a Windows machine and installs the appropriate driver.

# Example Playbook

```yml
# Installs Graphics Tools on all Windows machines.
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_win_gpu_drivers
```

# Updating Driver

## Radeon Vega Drivers

Download the Windows 10 - 64-Bit Edition drivers from
[Radeon RX Vega 56 Drivers & Support](https://www.amd.com/en/support/graphics/radeon-rx-vega-series/radeon-rx-vega-series/radeon-rx-vega-56).
Make sure to download the **Adrenalin Edition**.
These are downloaded as a self-extracting executable that needs to
be converted into a ZIP archive - See [ZIP Archive Creation](#ZIP-Archive-Creation)
below.

Next upload the newly created ZIP archive to the GCS bucket described in
[download_win_package_src](https://skia.googlesource.com/buildbot/+/HEAD/skolo/ansible/switchboard/roles/download_win_package_src/).

For example:

```console
$ gsutil cp whql-amd-software-adrenalin-edition-23.5.2-win10-win11-may31.zip \
    gs://skia-buildbots/skolo/win/win_package_src
```

## ZIP Archive Creation

```sh
$ skolo/bash/self_extracting_exe_to_zip.sh setup.exe
```

This will create a new ZIP archive beside the self-extracting executable.
(e.g. "setup.zip").

# Running Playbook

```console
$ cd skolo/ansible
$ ansible-playbook switchboard/install_win_gpu_drivers.yml \
  --limit skia-e-win-263
```
