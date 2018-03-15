const puppeteer = require('puppeteer');
const express = require('express')
const app = express();

// Really only need to serve 3 files:
//  driver.html, lottie.js, and the json file.

app.get('/', (req, res) => res.send('Hello World!'))

app.listen(3000, () => console.log('Example app listening on port 3000!'))

if (process.argv.length != 3) {
  console.error("You must supply a lottie JSON filename.");
  process.exit();
}

async function wait(ms) {
    await new Promise(resolve => setTimeout(() => resolve(), ms));
    console.log('waited', ms);
    return ms;
}

async function driveBrowser() {
  console.log(process.argv[2]);
  const browser = await puppeteer.launch();
  const page = await browser.newPage();
  await page.goto('http://localhost:8080/driver.html', {waitUntil: 'networkidle2'});
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
