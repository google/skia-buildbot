Pushable Base Snapshot Generator
================================

Run:

    $ ./vm_create_instance.sh

to set up a GCE instance and have it configured how pushable base images
should be configured, via the startup-script.sh. Once the startup-script
finishes you can take a new snapshot of the image using the Cloud Console
web page. The snapshot name should be:

    skia-systemd-pushable-base

Once the snapshot is taken you can close down the instance by running:

    $ ./vm_delete_instance.sh

Base Image
----------

The base image will be a Debian Testing installation. Apps installed are:

    1. git
    1. pulld
    1. unattended-upgrades

The unattended-upgrades package is installed and configured to do unattended
upgrades, i.e. the instance will run apt update/upgrade on a daily basis and
reboot at midnight if necessary.
