# This image can be found by visiting https://gcr.io/cloud-marketplace/google/debian10. On that
# page, you will find a list of images sorted by update date. Clicking on an image takes you to the
# image details page, where you can find its SHA256 hash.
FROM gcr.io/cloud-marketplace/google/debian12@sha256:f13f9826c524466f844e74534b8add32f51acd2be428b5dee782fe9086e59861
RUN apt-get update

# Needed by rules_go.
RUN apt-get install -y clang

# Needed by the Cloud Emulators.
RUN apt-get install -y openjdk-17-jdk-headless

# Needed by depot_tools.
RUN apt-get install -y curl

# zip is necessary for the undeclared outputs of tests running on RBE to show up under
# //_bazel_testlogs/path/to/test/test.outputs/outputs.zip. This is the mechanism we use to
# extract screenshots taken by Puppeteer tests. See b/147694106.
RUN apt-get install -y zip

# Libraries needed for Chrome and Chromium.
#
# We arrived at the below list of libraries by repeatedly running an arbitrary Karma test
# on RBE and installing any missing libraries reported by Chrome.
RUN apt-get install -y \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcairo-gobject2 \
    libcairo2 \
    libdatrie1 \
    libdrm2 \
    libepoxy0 \
    libfribidi0 \
    libgbm1 \
    libgdk-pixbuf2.0-0 \
    libgtk-3-0 \
    libnss3 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libpangoft2-1.0-0 \
    libpixman-1-0 \
    libthai0 \
    libwayland-client0 \
    libwayland-cursor0 \
    libwayland-egl1 \
    libwayland-server0 \
    libx11-6 \
    libx11-xcb1 \
    libxau6 \
    libxcb-render0 \
    libxcb-shm0 \
    libxcb1 \
    libxcomposite1 \
    libxcursor1 \
    libxdamage1 \
    libxdmcp6 \
    libxext6 \
    libxfixes3 \
    libxi6 \
    libxinerama1 \
    libxkbcommon0 \
    libxrandr2 \
    libxrender1 \
    libxshmfence1
