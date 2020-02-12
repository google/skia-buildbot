Driving iOS tests with Raspberry Pi's
=====================================

The tools in this directory allow to run iOS apps
built on a Mac to be run by iOS devices attached
to a Raspberry Pi.

The ideal workflow is to run the 'build.sh' script on
the target platform and then manually install the generated
package for local testing or copy it for remote installation.

When installing in a chroot, the post-installation script may
fail because it is unable to reload systemd. This shouldn't
cause any problems and is just an artifact of using the
shared release.sh script.

install-dependencies.sh
Install all necessary system packages via apt-get.

build.sh
Checks out the libimobiledevice
family of tools (see http://www.libimobiledevice.org/)
and builds them. This is intended to be run a raspberry pi,
but should work on most (Debian based) Linux distributions.

package.sh
Builds a Debian package of the tools. This
should work on x86 desktops as well as Raspberry Pi's.

clean.sh
Removes all files created by build.sh.

Deployment Notes:
=================
To make the USB connection to the iOS device more stable
it is recommended to remove the 'libmtp9' and 'libmtp-common'
packages and their dependencies.
