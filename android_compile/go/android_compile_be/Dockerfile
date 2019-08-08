# gcr.io/skia-public/basedebian:testing-slim python installation conflicts with
# the one expected by android compilation.
FROM debian:9.5

USER root

# Install necessary packages (from https://source.android.com/setup/initializing).
RUN apt-get update && apt-get install -y wget procps ccache gpg curl vim \
    git-core gnupg flex bison gperf build-essential zip curl zlib1g-dev \
    gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
    x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev \
    libxml2-utils xsltproc unzip python openjdk-8-jdk

# TODO(rmistry): Figure out a way to not do this hack.
RUN ln -s /usr/lib/x86_64-linux-gnu/libncursesw.so.6.1 /usr/lib/libtinfo.so.5 \
    && ln -s /usr/lib/x86_64-linux-gnu/libncursesw.so.6.1 /usr/lib/libncurses.so.5

# Add user so we don't have to run as root (prevents us from over-writing files in /SRC)
RUN groupadd -g 2000 skia \
    && useradd -u 2000 -g 2000 skia \
    && mkdir -p /home/skia \
    && chown -R skia:skia /home/skia

USER skia

# Install repo tool.
RUN mkdir /home/skia/bin \
    && wget https://storage.googleapis.com/git-repo-downloads/repo -O /home/skia/bin/repo \
    && chmod a+x /home/skia/bin/repo

# Add repo tool to PATH.
ENV PATH /home/skia/bin:$PATH

# Set git configs required for the repo tool to not prompt.
RUN git config --global color.ui true

COPY . /

ENV JAVA_HOME /usr/lib/jvm/java-1.8.0-openjdk-amd64
ENV PATH /home/skia/bin:$PATH

ENTRYPOINT ["/usr/local/bin/android_compile_be"]
