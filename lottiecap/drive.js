const puppeteer = require('puppeteer');
const express = require('express');
const fs = require('fs');

if (process.argv.length != 3) {
  console.error("You must supply a lottie JSON filename.");
  process.exit();
}

let lottieJS = fs.readFileSync('node_modules/lottie-web/build/player/lottie.min.js', 'utf8');
let driverHTML = fs.readFileSync('driver.html', 'utf8');
let lottieJSON = fs.readFileSync(process.argv[2], 'utf8');

const app = express();
// Really only need to serve 3 files:
// driver.html, lottie.js, and the json file.
app.get('/', (req, res) => res.send(driverHTML));
app.get('/lottie.js', (req, res) => res.send(lottieJS));
app.get('/lottie.json', (req, res) => res.send(lottieJSON));
app.listen(8081, () => console.log('Example app listening on port 8081!'))

async function wait(ms) {
    await new Promise(resolve => setTimeout(() => resolve(), ms));
    console.log('waited', ms);
    return ms;
}

async function driveBrowser() {
  const browser = await puppeteer.launch();
  const page = await browser.newPage();
  await page.goto('http://localhost:8081/', {waitUntil: 'networkidle2'});
  await wait(5 * 1000);
  await page.screenshot({
    path: 'page.png',
    clip: {
      x: 0,
      y: 0,
      width: 1000,
      height: 1000,
    },
  });

  await browser.close();
  process.exit();
}

driveBrowser();
