# This bash file is intended to be used for building .deb release
# files to be used by pull and push. To use this file just
# create your own bash file in which you define the APPNAME and DESCRIPTION
# vars and the copy_release_files() function which copies all the files
# needed in the distribution in ${ROOT}. Then source this file
# after those definitions. The resulting .deb will be uploaded to Google
# Storage with the correct metadata.
#
# Follow Debian conventions for file locations. For example:
#
# HTML Template files in /usr/local/share/${APPNAME}/.
# Binaries in /usr/local/bin/${APPNAME}.
# Small read/write files in /var/local/${APPNAME}/.
# Config files in /etc/${APPNAME}/.
#
# The first command line argument to the calling script
# will be used as the 'note' for the release package.
#
# For more details see ../push/DESIGN.md.

set -x

ROOT=`mktemp -d`
OUT=`mktemp -d`

# Create all directories here, so their perms can be set correctly.
mkdir --parents ${ROOT}/DEBIAN

# Set directory perms.
sudo chmod 755 -R ${ROOT}

# Create the control files that describes this deb.
echo 2.0 > ${ROOT}/DEBIAN/debian-binary
cat <<-EOF > ${ROOT}/DEBIAN/control
	Package: skia-${APPNAME}
	Version: 1.0
	Architecture: amd64
	Maintainer: ${USERNAME}@${HOST}
	Priority: optional
	Description: ${DESCRIPTION}
EOF

copy_release_files

# Build the debian package.
sudo dpkg-deb --build ${ROOT} ${OUT}/${APPNAME}.deb

# Upload the package to right location in Google Storage.
DATETIME=`date --utc "+%Y-%m-%dT%H:%M:%SZ"`
HASH=`git rev-parse HEAD`
USERID=${USER}@${HOSTNAME}
if git diff-index --quiet HEAD --; then
  DIRTY=false
else
  DIRTY=true
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
