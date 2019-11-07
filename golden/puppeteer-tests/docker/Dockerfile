FROM node:12.13

# Install Chrome and some missing dependencies. Copied verbatim from:
# https://github.com/GoogleChrome/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker
RUN wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list' \
    && apt-get update \
    && apt-get install -y google-chrome-stable fonts-ipafont-gothic fonts-wqy-zenhei fonts-thai-tlwg fonts-kacst fonts-freefont-ttf \
      --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

# Prevent Puppeteer (which we'll install with "npm install") from downloading
# Chromium as we will be using the Chrome package from the previous step.
ENV PUPPETEER_SKIP_CHROMIUM_DOWNLOAD true

# Output directory. Screenshots and other test output will be stored here.
RUN mkdir /out

WORKDIR /src

# This runs "npm install" only if package{-lock}.json changed.
COPY package.json package-lock.json ./
RUN npm install

# Any dependencies should be explicitly copied into the image.
COPY webpack.config.js ./
COPY modules modules/
COPY pages pages/
COPY puppeteer-tests puppeteer-tests/

WORKDIR puppeteer-tests

# Runs all tests under directory "puppeteer-tests".
CMD ["npx", "mocha"]
