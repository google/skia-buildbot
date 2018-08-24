
Serving image is based off of [https://github.com/ehough/docker-nfs-server], which has good documentation
for the various settings.

This is intended to be run in the Skolo on rpi-master/spare.


First time setup to make the nfs kernel modules available:

    sudo modprobe nfs
    sudo modprobe nfsd

Authenticate with GCS so the docker image can be pulled:

    python print_auth_token.py | docker login -u oauth2accesstoken --password-stdin https://gcr.io


    EXPORTED_IP="192.168.1.100"
    VIRTUAL_INTERFACE="eno1:1"
    sudo ifconfig $VIRTUAL_INTERFACE $EXPORTED_IP
    docker run -d --privileged \
    -p $EXPORTED_IP:2049:2049   -p $EXPORTED_IP:2049:2049/udp \
    -p $EXPORTED_IP:111:111     -p $EXPORTED_IP:111:111/udp     \
    -p $EXPORTED_IP:32765:32765 -p $EXPORTED_IP:32765:32765/udp \
    -p $EXPORTED_IP:32767:32767 -p $EXPORTED_IP:32767:32767/udp \
    gcr.io/skia-public/serve-rpi-image:latest

Nothing major changes on the cmdline.txt of the RPIs

    smsc95xx.turbo_mode=N dwc_otg.lpm_enable=0 console=ttyAMA0,115200 console=tty1 root=/dev/nfs nfsroot=192.168.1.100:/opt/prod/root,nfsvers=3 ip=192.168.1.YYY:192.168.1.100:192.168.1.1:255.255.255.0:skia-rpi-NNN:eth0:off:8.8.8.8:8.8.4.4 elevator=deadline fsck.repair=yes rootwait

Where YYY and NNN are statically configured for each RPI.

To build the image (to use a different rpi image, change Makefile):

    make build

To publish the image as latest and VERSION (see Makefile):

    make push


To debug the image locally (note we need to override the entrypoint to actually get a shell):

    docker run -it --privileged --entrypoint /bin/bash serve-rpi-image