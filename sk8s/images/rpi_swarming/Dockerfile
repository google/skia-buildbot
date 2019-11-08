FROM arm32v7/debian:bullseye-slim

COPY /usr/bin/qemu-arm-static /usr/bin

# Swarming MUST be run with one of five given user names, such as 'chrome-bot'.
RUN groupadd --gid 2000 chrome-bot && \
    useradd --create-home --uid 2000 --gid 2000 chrome-bot && \
    export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y python2.7 curl ca-certificates && \
    c_rehash
# Run c_rehash because https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=923479

COPY . /

# Swarming will crash w/o a .boto file.
RUN touch /home/chrome-bot/.boto

# Force swarming to recognize containerd as Docker.
RUN touch /.docker_env

CMD ["/start_swarming"]