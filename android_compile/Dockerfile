FROM gcr.io/skia-public/basedebian:testing-slim
#FROM debian:9.5

USER root

# Install necessary packages (from https://source.android.com/setup/initializing).
RUN apt-get update && apt-get install -y wget procps

#RUN apt-get update \
#    && apt-get install -y openjdk-8-jdk git-core gnupg flex bison gperf \
#    build-essential zip curl zlib1g-dev gcc-multilib g++-multilib \
#    libc6-dev-i386 lib32ncurses5-dev x11proto-core-dev libx11-dev lib32z-dev \
#    ccache libgl1-mesa-dev libxml2-utils xsltproc unzip

#Add user so we don't have to run as root (prevents us from over-writing files in /SRC)
# RUN groupadd -g 2000 skia \
#     && useradd -u 2000 -g 2000 skia \
#     && mkdir -p /home/skia \
#     && chown -R skia:skia /home/skia

# Create a 200G ram disk for ccache.
#RUN mkdir -p /mnt/pd0/ccache \
#    && mount -t tmpfs -o size=200G,nr_inodes=10M,mode=1777 tmpfs /mnt/pd0/ccache \
#    && chown skia:skia -R /mnt/pd0/ccache

# Add mounting instructions to fstab so that the ram disk remounts on reboot.
RUN echo "tmpfs /mnt/pd0/ccache tmpfs nodev,nosuid,noexec,nodiratime,size=200G 0 0" | tee -a /etc/fstab

USER skia

# Install repo tool.
RUN mkdir /home/skia/bin \
    && wget https://storage.googleapis.com/git-repo-downloads/repo -O /home/skia/bin/repo \
    && chmod a+x /home/skia/bin/repo

# Install gcompute-tools.
RUN git clone https://gerrit.googlesource.com/gcompute-tools ~/gcompute-tools

# Add repo and gcompute-tools to PATH.
RUN echo '# Add Android tools to PATH' >> ~/.bashrc \
    && echo 'export PATH="/home/skia/bin:$PATH"' >> ~/.bashrc \
    && echo 'export PATH="/home/skia/gcompute-tools:$PATH"' >> ~/.bashrc

# Set git configs required for the repo tool to not prompt.
RUN git config --global color.ui true

# Do the rest!

COPY . /

ENTRYPOINT ["/usr/local/bin/android_compile"]
