Drive iOS tests with Raspberry Pi's
===================================

The tools in this directory allow to run iOS apps
built on a Mac to be run by iOS devices attached
to a Raspberry Pi.

install_libimobiledevice.sh: Checkes out the libimobiledevice
family of tools (see http://www.libimobiledevice.org/)
and builds them. This is intended to be run a raspberry pi,
but should work on most (Debian based) Linux distributions.

build_relase.sh: Builds a Debian package of the tools.
Intended to be run on a Raspberry Pi.

