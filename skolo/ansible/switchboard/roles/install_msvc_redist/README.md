# Role Name

`install_msvc_redist`

# Description

Installs the Microsoft Visual C++ Redistributable.

The current versions were created using:

```console
$ vc_redist_version="17"
$ curl -L -o vc_redist_${vc_redist_version}.x64.exe https://aka.ms/vs/${vc_redist_version}/release/vc_redist.x64.exe
$ curl -L -o vc_redist_${vc_redist_version}.x86.exe https://aka.ms/vs/${vc_redist_version}/release/vc_redist.x86.exe
$ gsutil cp vc_redist_${vc_redist_version}.x64.exe gs://skia-buildbots/skolo/win/win_package_src
$ gsutil cp vc_redist_${vc_redist_version}.x86.exe gs://skia-buildbots/skolo/win/win_package_src
$ rm vc_redist_${vc_redist_version}.x64.exe
$ rm vc_redist_${vc_redist_version}.x86.exe
```

Assuming that Microsoft keeps the download paths the same, you should be able to
update the version by copy/pasting the above and modifying vc_redist_version,
and then update the file names in main.yml.

# Example Playbook

```yml
# Installs the Microsoft Visual C++ Redistributable.
- hosts: all_win
  user: chrome-bot
  gather_facts: yes

  roles:
    - install_msvc_redist
```
