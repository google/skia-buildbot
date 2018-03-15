const puppeteer = require('puppeteer');

function resolveAfter2Seconds() {
  return new Promise(resolve => {
    setTimeout(() => {
      resolve();
    }, 2000);
  });
}

(async() => {
const browser = await puppeteer.launch();
const page = await browser.newPage();
await page.goto('http://localhost:8080/driver.html', {waitUntil: 'networkidle2'});
await resolveAfter2Seconds();
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
})();
