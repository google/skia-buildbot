// This is just a sample test to verify that Puppeteer works inside Docker. It
// shows how to use Puppeteer to query the DOM and take screenshots.
//
// TODO(lovisolo): Remove after we have a few real tests.

const expect = require('chai').expect;
const express = require('express');
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');

describe('puppeteer', function() {
  let browser, page, server;

  // DO NOT SUBMIT before removing the console.log statements below.
  before(async () => {
    console.log('entering before()');
    console.log('calling startTestServer()');
    server = await startTestServer();
    console.log('calling launchBrowser()');
    browser = await launchBrowser();
    console.log('leaving before()');
  });

  after(async () => {
    console.log('entering after()');
    console.log('calling browser.close()');
    await browser.close();
    console.log('calling server.close()');
    await server.close();
    console.log('leaving after()');
  });

  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  it('queries the DOM', async () => {
    await page.goto(`http://localhost:${server.address().port}`);
    expect(await page.$eval('h1', (el) => el.innerText)).to.equal('hello');
    expect(await page.$eval('p', (el) => el.innerText)).to.equal('world');
  });

  it('takes screenshots', async () => {
    await page.goto(`http://localhost:${server.address().port}`);
    await page.screenshot({path: path.join(outputDir(), 'screenshot.png')});
  });
});

// Starts an Express server on a random, unused port. Serves a test page.
const startTestServer = () => {
  const app = express();
  app.get('/', (_, res) => {
    res.send('<html><body><h1>hello</h1><p>world</p></body></html>');
  });
  return new Promise((resolve) => {
    console.log('calling app.listen()');
    const server = app.listen(0, () => {
      console.log('resolving startTestServer() promise');
      resolve(server);
    });
  });
};

// TODO(lovisolo): Extract out the functions below into a file named e.g.
//                 "testbed.js" under directory "puppeteer-tests".

const inDocker = () => fs.existsSync('/.dockerenv');

const launchBrowser = () => puppeteer.launch(inDocker() ? {
  args: ['--disable-dev-shm-usage', '--no-sandbox'],
} : {});

const outputDir =
    () => inDocker()
        ? '/out'
        // Resolves to $SKIA_INFRA_ROOT/golden/puppeteer-tests/output.
        : path.join(__dirname, '..', 'output');
