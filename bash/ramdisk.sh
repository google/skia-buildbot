# Utilities for creating a service account key and securely moving into a
# Kubernetes secret.

set -e

mkdir /tmp/ramdisk
echo "sudo is needed to create the ramdisk."
sudo mount  -t tmpfs -o size=10m tmpfs /tmp/ramdisk

function finish {
  sleep 2
  sudo umount /tmp/ramdisk
  rmdir /tmp/ramdisk
}
trap finish EXIT
