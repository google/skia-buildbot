#!/bin/sh -eu

# Set the exports file and display
#echo "Export points:"
#echo "$EXPORTED_DIRECTORY *($EXPORT_SETTINGS)" | tee /etc/exports

# mount the disk image
# umount /exported/root
# umount /dev/loop0
# mount -v -r -o offset=50331648 -t ext4 /images/disk.img /exported/root

# Start the server
echo -e "\n- Initializing nfs server.."
/usr/sbin/exportfs -r
/sbin/rpcbind --
/usr/sbin/rpc.nfsd |:
/usr/sbin/rpc.mountd -F
