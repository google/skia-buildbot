FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && apk add --no-cache nfs-utils bash

# RUN mkdir /images && mkdir -p /exported/root
# RUN mkdir -p /exported



# COPY images/raspilite.img /images/disk.img
COPY exports /etc/exports
COPY run-nfs-server.sh /usr/local/bin/run-nfs-server.sh
#COPY ./sd_card/root /exported/root

# ENV EXPORTED_DIRECTORY /mnt
# ENV EXPORT_SETTINGS rw,fsid=0,async,no_subtree_check,no_auth_nlm,insecure,no_root_squash,crossmnt

EXPOSE 2049/tcp
#EXPOSE 111/tcp
#EXPOSE 111/udp

ENTRYPOINT [ "/usr/local/bin/run-nfs-server.sh"]
