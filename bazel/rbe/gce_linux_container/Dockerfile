# When adding or updating Debian packages to this container, please make the version explicit (e.g.
# prefer `apt-get install foo=1.2.3` over `apt-get install foo`). This is to reach at least SLSA
# level 1 in that we know exactly what versions of the binaries are installed on the images we used
# to build things (https://slsa.dev/spec/v0.1/levels#detailed-explanation).

# This image can be found by visiting https://gcr.io/cloud-marketplace/google/debian10. On that
# page, you will find a list of images sorted by update date. Clicking on an image takes you to the
# image details page, where you can find its SHA256 hash.
FROM gcr.io/cloud-marketplace/google/debian10@sha256:96a0145e8bb84d6886abfb9f6a955d9ab3f8b1876b8f7572273598c86e902983
RUN apt-get update

# Needed by rules_go.
RUN apt-get install -y clang-11=1:11.0.1-2~deb10u1
RUN ln -s /usr/bin/clang-11 /usr/bin/clang

# Needed by the Cloud Emulators.
RUN apt-get install -y openjdk-11-jdk-headless=11.0.15+10-1~deb10u1

# Needed by depot_tools.
RUN apt-get install -y curl=7.64.0-4+deb10u2

# zip is necessary for the undeclared outputs of tests running on RBE to show up under
# //_bazel_testlogs/path/to/test/test.outputs/outputs.zip. This is the mechanism we use to
# extract screenshots taken by Puppeteer tests. See b/147694106.
RUN apt-get install -y zip=3.0-11+b1

# Libraries needed for Chrome and Chromium.
#
# We arrived at the below list of libraries by repeatedly running an arbitrary Karma test
# on RBE and installing any missing libraries reported by Chrome.
RUN apt-get install -y \
    libatk-bridge2.0-0=2.30.0-5 \
    libatk1.0-0=2.30.0-2 \
    libatspi2.0-0=2.30.0-7 \
    libcairo-gobject2=1.16.0-4+deb10u1 \
    libcairo2=1.16.0-4+deb10u1 \
    libdatrie1=0.2.12-2 \
    libdrm2=2.4.97-1 \
    libepoxy0=1.5.3-0.1 \
    libfribidi0=1.0.5-3.1+deb10u1 \
    libgbm1=18.3.6-2+deb10u1 \
    libgdk-pixbuf2.0-0=2.38.1+dfsg-1 \
    libgtk-3-0=3.24.5-1 \
    libnss3=2:3.42.1-1+deb10u5 \
    libpango-1.0-0=1.42.4-8~deb10u1 \
    libpangocairo-1.0-0=1.42.4-8~deb10u1 \
    libpangoft2-1.0-0=1.42.4-8~deb10u1 \
    libpixman-1-0=0.36.0-1 \
    libthai0=0.1.28-2 \
    libwayland-client0=1.16.0-1 \
    libwayland-cursor0=1.16.0-1 \
    libwayland-egl1=1.16.0-1 \
    libwayland-server0=1.16.0-1 \
    libx11-6=2:1.6.7-1+deb10u2 \
    libx11-xcb1=2:1.6.7-1+deb10u2 \
    libxau6=1:1.0.8-1+b2 \
    libxcb-render0=1.13.1-2 \
    libxcb-shm0=1.13.1-2 \
    libxcb1=1.13.1-2 \
    libxcomposite1=1:0.4.4-2 \
    libxcursor1=1:1.1.15-2 \
    libxdamage1=1:1.1.4-3+b3 \
    libxdmcp6=1:1.1.2-3 \
    libxext6=2:1.3.3-1+b2 \
    libxfixes3=1:5.0.3-1 \
    libxi6=2:1.7.9-1 \
    libxinerama1=2:1.1.4-2 \
    libxkbcommon0=0.8.2-1 \
    libxrandr2=2:1.5.1-1 \
    libxrender1=1:0.9.10-1 \
    libxshmfence1=1.3-1
