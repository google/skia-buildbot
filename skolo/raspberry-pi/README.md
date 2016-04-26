# Skolo + Raspberry Pi

Skolo has a fleet of Raspberry Pis that are attached to one (and only one) Android device.

This directory contains scripts for making that all happen.

This is meant to be a detailed description, with [this design doc](https://docs.google.com/document/d/1bbEfQSZvAk5yIpq4Ey1gGgGQscdO9KB0Jfe962XcowA/edit#)
acting as a high level overview.  If the current setup is lost in a fire and this document is the
only thing remaining, it should be sufficient to fix Skolo.

## Setting up the Server
The server can theoretically be run on any OS that supports NSF mounting.
It is currently running Ubuntu 14.04, although that is not for any special reason other than it was
what was stable LTS at the time.
Make the username chrome-bot, use the buildbot password from Valentine.
I also suggest putting the jump host's ssh public key in ~/.ssh/authorized_keys.

```
sudo apt-get update && sudo apt-get upgrade
sudo apt-get install ansible git
[clone this repo]
cd skolo/raspberry-pi
ansible-playbook -i "localhost," -c local setup_master.yml

ifconfig
# Write down the ip address for this machine if only using one machine.  Otherwise, use the load balancer's.
```

I have written the ansible commands to be executed locally, although the ansible playbooks lend
themselves very well to being executed from a remote machine.


## Building the image (from scratch)
  - Download and uncompress the [latest raspbian "lite" image](https://www.raspberrypi.org/downloads/raspbian/).  Last known good version 2016-03-18-raspbian-jessie-lite.img
  - run `fdisk -lu raspian-jessie-lite.img`  The start columns are important, as they tell us where the boot and root partitions of the image start.  Update the playbooks `setup_for_chroot.yml` and `start_serving_image.yml` to have offset_root and offset_boot be correct.  offset_root and offset_boot are in bytes, so multiply by the sector size, probably 512.
  - `ansible-playbook -i "localhost," -c local setup_for_chroot.yml`  It is largely inspired by http://raspberrypi.stackexchange.com/a/856
  - We will now chroot into the image and, using basically "an ARM emulator", run commands on the image.  As much as possible has been scripted into finalize_image.yml, but some manual stuff must still be done.  I could not figure out how to get Ansible to execute commands inside the chroot.

```
sudo chroot /opt/raspberrypi/root/
# You should now be in a different filesystem.  Check this with pwd.  You should be in /

# First we fix the keyboard layout
dpkg-reconfigure keyboard-configuration
# Select the 104 keyboard option.  Then, when it prompts you for locale, go down to Other and select US

# give root user a password, just in case.  Use/Put the password in the password manager.
passwd root

# Make our user for swarming
adduser chrome-bot
# Use/Put the password in the password manager.

# Load some android public/private keys for python-adb to use into /home/chrome-bot/.android
# Download from https://pantheon.corp.google.com/storage/browser/skia-buildbots/rpi_auth/?project=google.com:skia-buildbots
# then use chown to make chrome-bot own them.

# Ctrl+D to exit chroot
```
  - `./setup-swarming.sh`  This will do all automatic, idempotent setup that is possible.  If Ansible can be configured to act inside a chroot, this should be ported to Ansible.
  - `ansible-playbook -i "localhost," -c local finalize_image.yml`  finalize_image copies any scripts we need, changes any additional files that can be done in an automated way.
  - The mounted image file has been receiving the changes this entire time, no need to `dd` anything to back it up.  Current backups are in gs://skia-images/Swarming

## Begin serving the image
`ansible-playbook -i "localhost," -c local start_serving_image.yml`

## Adding a new Raspberry PI into the swarm
This is also quite straight-forward.
 1. Assemble Raspberry PI hardware.  Connect Ethernet cable.
 2. Insert blank SD card into one of the masters.
 3. `ansible-playbook -i "localhost," -c local format_new_card.yml`  Type in static IP address and hostname suffix.
 4. Insert SD card into new pi.  Turn on.

The SD card is partitioned to have two primary partitions and one extended partitions with 3 logical partitions.
 - `/boot` is a small primary partition that has the boot code and a key file called `cmdline.txt`.  That file is changed to mount via nfs the /root file system and boot from it.  This took a lot of tweaking to get right, and there was no authoritative source on the matter.  I had to use a few different sources and a bit of dumb luck to get the configuration just right.
 - `/tmp`, `/var`, `/home` are all logical partitions on the extended partition.  These three partitions are mounted on the SD card so the RPI can function properly and we have persistent access to logs. They are each 1G, but this can be changed if needed.
 - `/b` is a primary partition that takes up all the rest of the space.  It is used as the workspace for swarming.