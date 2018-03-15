/**
 * Command line application to build a 5x5
 * filmstrip from a Lottie file in the browser
 * and then exporting that filmstrip in a
 * 1000x1000 pixel PNG.
 *
 * Usage:
 *   node ./capture_lottie.js `filename`
 *
 * The filename is the name of the Lottie JSON file.
 *
 * The server uses port 8081.
 *
 * The captured image is written to `filmstrip.png`.
 *
 */
const puppeteer = require('puppeteer');
const express = require('express');
const fs = require('fs');

if (process.argv.length != 3) {
  console.error("You must supply a Lottie JSON filename.");
  process.exit(1);
}

// Start up a web server to serve the three files we need.
let lottieJS = fs.readFileSync('node_modules/lottie-web/build/player/lottie.min.js', 'utf8');
let driverHTML = fs.readFileSync('driver.html', 'utf8');
let lottieJSON = fs.readFileSync(process.argv[2], 'utf8');

const app = express();
app.get('/', (req, res) => res.send(driverHTML));
app.get('/lottie.js', (req, res) => res.send(lottieJS));
app.get('/lottie.json', (req, res) => res.send(lottieJSON));
app.listen(8081, () => console.log('- Local web server started.'))

// Utiltity function.
async function wait(ms) {
    await new Promise(resolve => setTimeout(() => resolve(), ms));
    return ms;
}

// Drive chrome to load the web page from the server we have running.
async function driveBrowser() {
  console.log('- Launching chrome in headless mode.');
  const browser = await puppeteer.launch();
  const page = await browser.newPage();
  console.log('- Loading our Lottie exercising page.');
  await page.goto('http://localhost:8081/', {waitUntil: 'networkidle2'});
  console.log('- Waiting for all the tiles to be drawn.');
  await page.waitForFunction('window._tileCount === 24');
  console.log('- Taking screenshot.');
  await page.screenshot({
    path: 'filmstrip.png',
    clip: {
      x: 0,
      y: 0,
      width: 1000,
      height: 1000,
    },
  });
  await browser.close();
  // Need to call exit() because the web server is still running.
  process.exit(0);
}

driveBrowser();
