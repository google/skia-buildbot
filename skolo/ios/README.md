# Driving iOS tests with Raspberry Pi's

The tools in this directory allow iOS apps built on a Mac run on iOS devices
attached to a Raspberry Pi.

The ideal workflow is to run the 'build.sh' script on the target platform and
then manually install the generated package for local testing or copy it for
remote installation.

## Scripts

### install-dependencies.sh

Installs all packages necessary for development.

### build.sh

Checks out the libimobiledevice family of tools (see
http://www.libimobiledevice.org/) and builds them. This is intended to be run a
Raspberry Pi, but should work on most (Debian based) Linux distributions. We
roll 1 big package containing what several Debian-proper ones do, just to reduce
paperwork, since we have to build everything from source anyway atop our patched
libimobiledevice.

### package.sh

Builds a Debian package of the tools. This should work on x86 desktops as well
as Raspberry Pi's.

### clean.sh

Removes all files created by build.sh.

## Troubleshooting

When installing in a chroot, the post-installation script may fail because it is
unable to reload systemd. This shouldn't cause any problems and is just an
artifact of using the shared release.sh script.

udev is sometimes shy about triggering for devices that were already plugged in
before usbmuxd was installed. This happens when a previous version of usbmuxd
was terminated with `kill` rather than `systemctl stop usbmuxd`. Symptoms are
that the iOS device won't show up in `idevice_id -l` and usbmuxd won't be
running. Rebooting fixes it.

## Deployment Notes

To make the USB connection to the iOS device more stable it is recommended to
remove the 'libmtp9' and 'libmtp-common' packages and their dependencies.

## Version History

### 1.0

Original version, with Ben Wagner's patches, intended for use with pushk.

### 1.1

Same patches, rebased on the latest upstreams, a bit after the 1.3.0 release
of libimobiledevice. Hashes of subprojects:

- https://github.com/skia-dev/libimobiledevice.git @ bf5f66f7216b7147e36629cb0f698a41053bb854
- ideviceinstaller @ d5c37d657969a6c71ff965a3f17004a844449879
- ifuse @ 14839dcda4ec8c86f11372665c853dc4a294fa72
- libimobiledevice-glue @ 7c37434360f1c49975c286566efc3f0c935a84ef
- libplist @ cf7a3f3d7c06b197ee71c9f97eb9aa05f26d63b5
- libusbmuxd @ 2ec5354a6ff2ba5e2740eabe7402186f29294f79
- usbmuxd @ e3a3180b9b380ce9092ee0d7b8e9d82d66b1c261
