# FROM gcr.io/skia-public/basedebian:testing-slim
# PYTHONHOME=/usr/bin/ PYTHONPATH="/usr/lib/python2.7/:/usr/lib/python2.7/plat-x86_64-linux-gnu/" bash /tmp/compile.sh /mnt/pd0/checkouts/checkout_4/

FROM debian:9.5

USER root

RUN apt-get update && apt-get install -y wget procps ccache gpg curl vim \
    git-core gnupg flex bison gperf build-essential zip curl zlib1g-dev \
    gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
    x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev \
    libxml2-utils xsltproc unzip python sudo

# TODO(rmistry): Figure out a way to not do this hack.
RUN ln -s /usr/lib/x86_64-linux-gnu/libncursesw.so.6.1 /usr/lib/libtinfo.so.5 \
    && ln -s /usr/lib/x86_64-linux-gnu/libncursesw.so.6.1 /usr/lib/libncurses.so.5

# TODO(rmistry): Remove this if debian:9.5 doe snot work with the python thingy.
#Add user so we don't have to run as root (prevents us from over-writing files in /SRC)
RUN groupadd -g 2000 skia \
    && useradd -u 2000 -g 2000 skia \
    && mkdir -p /home/skia \
    && chown -R skia:skia /home/skia

# TODO(rmistry): Remove this.
# RUN echo "skia:skia" | chpasswd && adduser skia sudo

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
