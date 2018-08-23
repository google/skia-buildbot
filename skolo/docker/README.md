
Serving image is based off of [https://github.com/ehough/docker-nfs-server], which has good documentation
for the various settings.


First time setup to make the nfs kernel modules available:

    sudo modprobe nfs
    sudo modprobe nfsd

How to run the image, binding the

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

Where YYY and NNN are staticaly configured for each RPI.


To debug the image locally (note we need to override the entrypoint to actually get a shell):

    docker run -it --privileged --entrypoint /bin/bash serve-rpi-image