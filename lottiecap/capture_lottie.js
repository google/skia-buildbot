/**
 * Command line application to build a 5x5 filmstrip from a Lottie file in the
 * browser and then exporting that filmstrip in a 1000x1000 PNG.
 *
 */
const puppeteer = require('puppeteer');
const express = require('express');
const fs = require('fs');
const commandLineArgs = require('command-line-args');
const commandLineUsage= require('command-line-usage');

const opts = [
  {
    name: 'input',
    typeLabel: '{underline file}',
    description: 'The Lottie JSON file to process.'
  },
  {
    name: 'output',
    typeLabel: '{underline file}',
    description: 'The captured filmstrip PNG file to write. Defaults to filmstrip.png',
  },
  {
    name: 'port',
    description: 'The port number to use, defaults to 8081.',
    type: Number,
  },
  {
    name: 'help',
    alias: 'h',
    type: Boolean,
    description: 'Print this usage guide.'
  },
];

const usage = [
  {
    header: 'Lottie Filmstrip Capture',
    content: `Command line application to build a 5x5 filmstrip
from a Lottie file in the browser and then export
that filmstrip in a 1000x1000 PNG.`
  },
  {
    header: 'Options',
    optionList: opts,
  },
];

const options = commandLineArgs(opts);

if (!options.output) {
  options.output = 'filmstrip.png';
}
if (!options.port) {
  options.port = 8081;
}

if (options.help) {
  console.log(commandLineUsage(usage));
  process.exit(0);
}

if (!options.input) {
  console.error("You must supply a Lottie JSON filename.");
  console.log(commandLineUsage(usage));
  process.exit(1);
}

// Start up a web server to serve the three files we need.
let lottieJS = fs.readFileSync('node_modules/lottie-web/build/player/lottie.min.js', 'utf8');
let driverHTML = fs.readFileSync('driver.html', 'utf8');
let lottieJSON = fs.readFileSync(options.input, 'utf8');

const app = express();
app.get('/', (req, res) => res.send(driverHTML));
app.get('/lottie.js', (req, res) => res.send(lottieJS));
app.get('/lottie.json', (req, res) => res.send(lottieJSON));
app.listen(options.port, () => console.log('- Local web server started.'))

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
  await page.goto('http://localhost:' + options.port + '/', {waitUntil: 'networkidle2'});
  console.log('- Waiting for all the tiles to be drawn.');
  await page.waitForFunction('window._tileCount === 25');
  console.log('- Taking screenshot.');
  await page.screenshot({
    path: options.output,
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
