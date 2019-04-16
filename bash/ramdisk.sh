# Utilities for creating a service account key and securely moving into a
# Kubernetes secret.

set -e

mkdir -p /tmp/ramdisk
echo "sudo is needed to create the ramdisk."
sudo mount  -t tmpfs -o size=10m tmpfs /tmp/ramdisk

function finish {
  sleep 10
  sudo umount /tmp/ramdisk
  sleep 10
  rmdir /tmp/ramdisk
}
trap finish EXIT
