# Utilities for creating a service account key and securely moving into a
# Kubernetes secret.

mkdir /tmp/ramdisk
sudo mount  -t tmpfs -o size=10m tmpfs /tmp/ramdisk
cd /tmp/ramdisk

function finish {
  cd -
  sleep 1
  sudo umount /tmp/ramdisk
  rmdir /tmp/ramdisk
}
trap finish EXIT
