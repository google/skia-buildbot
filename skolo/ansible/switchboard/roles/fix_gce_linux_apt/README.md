# fix_gce_linux_apt

Fixes apt/apt-get problems on Linux GCE hosts. This role should be run immediately after machine
creation.

Based on
https://skia.googlesource.com/buildbot/+/bd000c01fd40ce58a8c4272e7d234fc289939972/go/gce/swarming/setup-script-linux.sh.

## Example playbook

    - name: Fix apt on Linux GCE bots.
      include_role:
        name: fix_gce_linux_apt
