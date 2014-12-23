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
# provided a debian package for, for example influxdb.
#
# DEPENDS
# -------
# If DEPENDS is specified it should be a list of dependencies that this package
# depends upon. Note that they will not be installed if missing. If you want
# packages installed beyond the base snapshot they that should be done in the
# startup script. See https://cloud.google.com/compute/docs/startupscript
#
# For more details see ../push/DESIGN.md.

set -x -e

ROOT=`mktemp -d`
OUT=`mktemp -d`

if [ "$#" -ne 1 ]
then
    echo "Usage: You must supply a message when building a release package."
    exit 1
fi

# Create all directories here, so their perms can be set correctly.
mkdir --parents ${ROOT}/DEBIAN

# Set directory perms.
sudo chmod 755 -R ${ROOT}

# Create the control files that describes this deb.
echo 2.0 > ${ROOT}/DEBIAN/debian-binary

cat <<-EOF > ${ROOT}/DEBIAN/control
	Package: skia-${APPNAME}
	Version: 1.0
	Depends: ${DEPENDS}
	Architecture: amd64
	Maintainer: ${USERNAME}@${HOST}
	Priority: optional
	Description: ${DESCRIPTION}
EOF

copy_release_files

if [ ! -v BYPASS_GENERATION ]
then
  # Build the debian package.
  sudo dpkg-deb --build ${ROOT} ${OUT}/${APPNAME}.deb
else
  # Just use the debian package that copy_release_files
  # placed in ${ROOT}/{APPNAME}.deb.
  OUT=${ROOT}
fi

# Upload the package to right location in Google Storage.
DATETIME=`date --utc "+%Y-%m-%dT%H:%M:%SZ"`
HASH=`git rev-parse HEAD`
USERID=${USER}@${HOSTNAME}
# Detect if we have unchecked in local changes, or if we are different from HEAD at origin/master.
git fetch
if  [ git diff-index --quiet HEAD -- ] || [ $(git rev-parse HEAD) != $(git rev-parse @{u}) ] ; then
  DIRTY=true
else
  DIRTY=false
fi
gsutil \
  -h x-goog-meta-appname:${APPNAME} \
  -h x-goog-meta-userid:${USERID} \
  -h x-goog-meta-hash:${HASH} \
  -h x-goog-meta-datetime:${DATETIME} \
  -h x-goog-meta-dirty:${DIRTY} \
  -h "x-goog-meta-note:$1" \
  cp ${OUT}/${APPNAME}.deb \
  gs://skia-push/debs/${APPNAME}/${APPNAME}:${USERID}:${DATETIME}:${HASH}.deb
