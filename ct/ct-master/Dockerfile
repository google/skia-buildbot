# We use debian:9.5 here instead of gcr.io/skia-public/basealpine
# because the isolate binary downloaded by go/isolate requires access to
# a library that the alpine image does not contain.
FROM debian:9.5

USER root

RUN apt-get update \
    && apt-get install -y git file ca-certificates python-django python-setuptools lua5.2 procps wget \
    && easy_install -U pip \
    && pip install -U crcmod

#Add user so we don't have to run as root (prevents us from over-writing files in /SRC)
RUN groupadd -g 2000 skia \
    && useradd -u 2000 -g 2000 skia \
    && mkdir -p /home/skia \
    && chown -R skia:skia /home/skia

# Create required dirs.
RUN mkdir --parents /b/storage/ && chown skia:skia /b/storage

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/poller"]
