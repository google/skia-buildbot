# This bash file is intended to be used for building .deb release files to be
# used by pull and push. To use this file just create your own bash file in
# which you define the APPNAME and DESCRIPTION vars and the
# copy_release_files() function which copies all the files needed in the
# distribution in ${ROOT}. Then source this file after those definitions. The
# resulting .deb will be uploaded to Google Storage with the correct metadata.
#
# Follow Debian conventions for file locations. For example:
#
# HTML Template files in    /usr/local/share/${APPNAME}/
# Binaries in               /usr/local/bin/${APPNAME}
# Small read/write files in /var/local/${APPNAME}/
# Config files in           /etc/${APPNAME}/
#
# The first command line argument to the calling script
# will be used as the 'note' for the release package.
#
# BYPASS_GENERATION
# -----------------
# If BYPASS_GENERATION is set then we know that the caller has created or
# copied in the debian file themselves, and we are to use it as-is. This is
# useful in cases where we are installing software that the author has already
# provided a debian package for, for example grafana.
#
# BYPASS_UPLOAD
# -------------
# If BYPASS_UPLOAD is set then the generated package will not be uploaded to
# to Google Storage. This is meant for debugging.
#
# DEPENDS
# -------
# If DEPENDS is specified it should be a list of dependencies that this package
# depends upon. Note that they will not be installed if missing. If you want
# packages installed beyond the base snapshot, do it in the startup script. See
# https://cloud.google.com/compute/docs/startupscript
#
# For more details see ../push/DESIGN.md.
#
# BREAKS and CONFLICTS
# --------------------
# These, if set, go into the Breaks and Conflicts control-file fields of the
# package and have the semantics described at
# https://www.debian.org/doc/debian-policy/ch-relationships.html#packages-which-
# break-other-packages-breaks.
#
# SYSTEMD
# -------
# If defined this should be a space separated list of *.service files that are
# to be enabled and started when the package is installed. The target system
# must support systemd. The service file(s) should be copied into
# /etc/systemd/system/*.service in your copy_release_files() function. A
# post-installation script will be run that enables and runs all such
# services.
#
# SYSTEMD_TIMER
# -------------
# If defined it is assumed to contain a systemd timer that will trigger a
# delayed setup and restart of the services. In this case the services
# identified in SYSTEMD are enabled, but not restarted.
# This is useful if a service needs to install a package via apt-get.
# With this delay a systemd unit is triggered after the package is installed.
#
# UDEV_LIB_RELOAD
# ---------------
# If defined the post-install script will reload the udev rules and
# call ldconfig to index added libraries.

set -x

ROOT=`mktemp -d`
OUT=`mktemp -d`
REL=$(dirname "$BASH_SOURCE")
USERID=${USER}@`hostname`

if [ "$#" -ne 1 ]
then
    echo "Usage: You must supply a message when building a release package."
    exit 1
fi

if [ ! -v FORCE_ARCH ]
then
  # Get the current architecture.
  ARCH=`dpkg --print-architecture`
else
  ARCH=$FORCE_ARCH
fi

# Create all directories here, so their perms can be set correctly.
mkdir --parents ${ROOT}/DEBIAN

# Set directory perms.
chmod 755 -R ${ROOT}

# Create the control files that describes this deb.
echo 2.0 > ${ROOT}/DEBIAN/debian-binary

if [ -v CONFLICTS ]
then
  CONFLICTS=$'\n'"Conflicts: ${CONFLICTS}"
fi

if [ -v BREAKS ]
then
  BREAKS=$'\n'"Breaks: ${BREAKS}"
fi

cat <<-EOF > ${ROOT}/DEBIAN/control
	Package: skia-${APPNAME}
	Version: ${VERSION:-1.0}
	Depends: ${DEPENDS}${BREAKS}${CONFLICTS}
	Architecture: ${ARCH}
	Maintainer: ${USERID}
	Priority: optional
	Description: ${DESCRIPTION}
EOF

# Either restart SYSTEMD or SYSTEMD_TIMER.
RESTART_TARGET="$SYSTEMD"
if [ -v SYSTEMD_TIMER ]; then
  RESTART_TARGET=${SYSTEMD_TIMER}
fi

# Generate the post-install file that wires up the services.
cat <<-EOF > ${ROOT}/DEBIAN/postinst
#!/bin/bash
INIT_SCRIPT="${INIT_SCRIPT}"
set -e -x
if [ -e /bin/systemctl ]
then
  /bin/systemctl daemon-reload
EOF

# Only call enable if there is something to enable.
if [ ! -z "$SYSTEMD" ]; then
  cat <<-EOF >> ${ROOT}/DEBIAN/postinst
  /bin/systemctl start ${SYSTEMD}
EOF
fi

# Only restart if there is a target defined.
if [ ! -z "$RESTART_TARGET" ]; then
  cat <<-EOF >> ${ROOT}/DEBIAN/postinst
  /bin/systemctl restart ${RESTART_TARGET}
EOF
fi

cat <<-EOF >> ${ROOT}/DEBIAN/postinst
elif [ ! -z "\$INIT_SCRIPT" ]
then
  update-rc.d \$INIT_SCRIPT enable
  service $INIT_SCRIPT start
fi
EOF

if [ -n "${UDEV_LIB_RELOAD}" ]; then
  cat <<-EOF >> ${ROOT}/DEBIAN/postinst
/bin/udevadm control --reload-rules
ldconfig

# add the usb group and user if they don't exist.
if ! getent group plugdev >/dev/null; then
        echo "Adding group plugdev"
        sudo addgroup --system plugdev
fi
if ! getent passwd usbmux >/dev/null; then
        echo "Adding user usbmux"
        sudo adduser --system --ingroup plugdev --no-create-home --gecos "usbmux daemon" usbmux
fi
EOF
fi

chmod 755 ${ROOT}/DEBIAN/postinst

copy_release_files

if [ ! -v BYPASS_GENERATION ]
then
  # Build the debian package.
  fakeroot dpkg-deb --build ${ROOT} ${OUT}/${APPNAME}.deb
else
  # Just use the debian package that copy_release_files
  # placed in ${ROOT}/{APPNAME}.deb.
  OUT=${ROOT}
fi

if [ ! -v BYPASS_UPLOAD ]
then
  # Upload the package to right location in Google Storage.
  DATETIME=`date --utc "+%Y-%m-%dT%H:%M:%SZ"`
  HASH=`git rev-parse HEAD`
  DIRTY=false
  GITSTATE=`${REL}/gitstate.sh`
  if [ "$GITSTATE" = "dirty" ]; then
    DIRTY=true
  fi
  gsutil \
    -h x-goog-meta-appname:${APPNAME} \
    -h x-goog-meta-userid:${USERID} \
    -h x-goog-meta-hash:${HASH} \
    -h x-goog-meta-datetime:${DATETIME} \
    -h x-goog-meta-dirty:${DIRTY} \
    -h "x-goog-meta-note:$1" \
    -h "x-goog-meta-services:$SYSTEMD" \
    cp ${OUT}/${APPNAME}.deb \
    gs://skia-push/debs/${APPNAME}/${APPNAME}:${USERID}:${DATETIME}:${HASH}.deb
else
  echo "Upload bypassed."
fi
