FROM ubuntu:18.04

USER root

RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y glusterfs-server

# setup entrypoint
# WORKDIR /root
CMD ["/bin/bash"]
