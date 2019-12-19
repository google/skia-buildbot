const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');

/**
 * Returns true if running from within a Docker container, or false otherwise.
 * @return {boolean}
 */
exports.inDocker = () => fs.existsSync('/.dockerenv');

/**
 * Launches a Puppeteer browser with the right platform-specific arguments.
 * @return {Promise}
 */
exports.launchBrowser =
    () => puppeteer.launch(
        // See
        // https://github.com/puppeteer/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker.
        exports.inDocker()
            ? { args: ['--disable-dev-shm-usage', '--no-sandbox'] }
            : {});

/**
 * Returns the output directory where tests should e.g. save screenshots.
 * Screenshots saved in this directory will be uploaded to Gold.
 * @return {string}
 */
exports.outputDir =
    () => exports.inDocker()
        ? '/out'
        // Resolves to $SKIA_INFRA_ROOT/golden/puppeteer-tests/output.
        : path.join(__dirname, '..', 'output');

/**
 * Takes a screenshot and saves it to the tests output directory to be uploaded
 * to Gold.
 * @param {Object} page Puppeteer page.
 * @param {string} testName Test name, e.g. "Test-Foo-Bar".
 * @return {Promise}
 */
exports.takeScreenshot =
    (page, testName) =>
        page.screenshot({
          path: path.join(exports.outputDir(), `${testName}.png`)
        });
