// This is just a sample test to verify that Puppeteer works inside Docker. It
// shows how to use Puppeteer to query the DOM and take screenshots.
//
// TODO(lovisolo): Remove after we have a few real tests.

const expect = require('chai').expect;
const express = require('express');
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');



// DO NOT SUBMIT before removing this.
let firstNs = 0;
function dbg(msg) {
  let hrtime = process.hrtime();
  let ns = hrtime[0] * 1000000000 + hrtime[1];
  if (firstNs == 0) firstNs = ns;
  let delta = (ns - firstNs) / 1000000;
  console.log(`DEBUG (t=${delta}): ${msg}`);
}



describe('puppeteer', function() {
  this.timeout(60000); // Increase timeout.

  let browser, page, server;

  before(async () => {
    dbg('entering before()');
    dbg('calling startTestServer()');
    server = await startTestServer();
    dbg('calling launchBrowser()');
    browser = await launchBrowser();
    dbg('leaving before()');
  });

  after(async () => {
    dbg('entering after()');
    dbg('calling browser.close()');
    await browser.close();
    dbg('calling server.close()');
    await server.close();
    dbg('leaving after()');
  });

  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  it('queries the DOM', async () => {
    dbg('entering "queries the DOM"');
    await page.goto(`http://localhost:${server.address().port}`);
    expect(await page.$eval('h1', (el) => el.innerText)).to.equal('hello');
    expect(await page.$eval('p', (el) => el.innerText)).to.equal('world');
    dbg('leaving "queries the DOM"');
  });

  it('takes screenshots', async () => {
    dbg('entering "takes screenshots"');
    await page.goto(`http://localhost:${server.address().port}`);
    await page.screenshot({path: path.join(outputDir(), 'screenshot.png')});
    dbg('leaving "takes screenshots"');
  });
});

// Starts an Express server on a random, unused port. Serves a test page.
const startTestServer = () => {
  const app = express();
  app.get('/', (_, res) => {
    res.send('<html><body><h1>hello</h1><p>world</p></body></html>');
  });
  return new Promise((resolve) => {
    dbg('calling app.listen()');
    const server = app.listen(0, () => {
      dbg('resolving startTestServer() promise');
      resolve(server);
    });
  });
};

// TODO(lovisolo): Extract out the functions below into a file named e.g.
//                 "testbed.js" under directory "puppeteer-tests".

const inDocker = () => {
  dbg('entering inDocker()');
  const retval = fs.existsSync('/.dockerenv');
  dbg('leaving inDocker()');
  return retval;
}

const launchBrowser = () => {
  dbg('entering launchBrowser()');
  const promise = puppeteer.launch(inDocker() ? {
    args: ['--disable-dev-shm-usage', '--no-sandbox'],
  } : {});
  dbg('leaving launchBrowser()');
  return promise;
};

const outputDir =
    () => inDocker()
        ? '/out'
        // Resolves to $SKIA_INFRA_ROOT/golden/puppeteer-tests/output.
        : path.join(__dirname, '..', 'output');
