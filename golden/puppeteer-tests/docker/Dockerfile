FROM node:12.13

# Install Chrome and fonts to support major charsets. This installs the
# necessary libraries to make the bundled version of Chromium that Puppeter
# installs work. Copied verbatim from:
# https://github.com/GoogleChrome/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker
RUN wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list' \
    && apt-get update \
    && apt-get install -y google-chrome-stable fonts-ipafont-gothic fonts-wqy-zenhei fonts-thai-tlwg fonts-kacst fonts-freefont-ttf \
      --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

# Output directory. Screenshots and other test output will be stored here.
RUN mkdir /out

WORKDIR /src

# This runs "npm ci" only if package{-lock}.json changed.
COPY package.json package-lock.json ./

# See https://blog.npmjs.org/post/171556855892/introducing-npm-ci-for-faster-more-reliable
RUN npm ci

# Any dependencies should be explicitly copied into the image.
COPY webpack.config.js ./
COPY modules modules/
COPY pages pages/
COPY puppeteer-tests puppeteer-tests/

WORKDIR puppeteer-tests

# Runs all tests under directory "puppeteer-tests".
CMD ["npx", "mocha"]
