const puppeteer = require('puppeteer');

const http = require('http');

const hostname = '127.0.0.1';
const port = 3000;

const server = http.createServer((req, res) => {
    res.statusCode = 200;
    res.setHeader('Content-Type', 'text/plain');
    res.end('Hello World!\n');
});

server.listen(port, hostname, () => {
  console.log(`Server running at http://${hostname}:${port}/`);
});

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
