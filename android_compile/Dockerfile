FROM gcr.io/skia-public/basedebian:testing-slim

USER root

RUN apt-get update && apt-get install -y wget procps ccache gpg curl vim \
    git-core gnupg flex bison gperf build-essential zip curl zlib1g-dev \
    gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
    x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev \
    libxml2-utils xsltproc unzip python

# TODO(rmistry): Is the below needed?
# Add mounting instructions to fstab so that the ram disk remounts on reboot.
RUN echo "tmpfs /mnt/pd0/ccache tmpfs nodev,nosuid,noexec,nodiratime,size=100G 0 0" | tee -a /etc/fstab

USER skia

# Install repo tool.
RUN mkdir /home/skia/bin \
    && wget https://storage.googleapis.com/git-repo-downloads/repo -O /home/skia/bin/repo \
    && chmod a+x /home/skia/bin/repo

# Add repo to PATH.
RUN echo '# Add Android tools to PATH' >> ~/.bashrc \
    && echo 'export PATH="/home/skia/bin:$PATH"' >> ~/.bashrc

# Set git configs required for the repo tool to not prompt.
RUN git config --global color.ui true

COPY . /

ENV PATH /home/skia/bin:$PATH

ENTRYPOINT ["/usr/local/bin/android_compile"]
