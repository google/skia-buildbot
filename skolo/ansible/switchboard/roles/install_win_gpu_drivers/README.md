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

## Nvidia Drivers

Download the Windows 11 drivers from https://www.nvidia.com/download/find.aspx

Search parameters:

- Product Type: **GeForce**
- Produce Series: **GeForce 900 Series**
- Product: GeForce **GTX 980 Ti**
- Operating System: **Windows 11**
- Windows Driver Type: **DCH**
- Language: **English (US)**
- Recommended/Beta: **Recommended/Certified**

Upload the drivers to the GCS bucket. For example:

For example:

```console
$ gsutil cp 536.40-desktop-win10-win11-64bit-international-dch-whql.exe \
    "gs://skia-buildbots/skolo/win/win_package_src/NVIDIA Graphics 536.40-desktop-win10-win11-64bit-international-dch-whql.exe"
```

Update the path in the playbook
`//skolo/ansible/switchboard/roles/install_win_gpu_drivers/tasks/nvidia.yml`.

## ZIP Archive Creation

Some playbooks require the driver to be converted from a self extracting
archive (`*.exe`) to a ZIP file (`*.zip`). This Linux script will automate
that conversion:

```sh
$ skolo/bash/self_extracting_exe_to_zip.sh setup.exe
```

This will create a new ZIP archive beside the self-extracting executable.
(e.g. "setup.zip").

# Running Playbook

## Performance Note

The driver playbooks download the **entire** driver source
folder from GCS for each host. This slows down updates and can even cause
failures if disk space is tight. Instead it is faster to download the folder
once and to pass the path to the downloaded drivers to the ansible script.

These can be downloaded as so:

```console
$ gsutil cp -r gs://skia-buildbots/skolo/win/win_package_src ~/
```

Then run the playbooks as so:

```console
$ cd skolo/ansible
$ ansible-playbook switchboard/install_win_gpu_drivers.yml \
   --extra-vars win_package_src=~/win_package_src
```

## Upgrading Single Host

```console
$ cd skolo/ansible
$ ansible-playbook switchboard/install_win_gpu_drivers.yml \
  --extra-vars win_package_src=~/win_package_src \
  --limit skia-e-win-205
```
