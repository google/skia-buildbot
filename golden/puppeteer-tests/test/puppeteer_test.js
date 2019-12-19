// This is just a sample test to verify that Puppeteer works inside Docker. It
// shows how to use Puppeteer to query the DOM and take screenshots.
//
// TODO(lovisolo): Remove after we have a few real tests.

const expect = require('chai').expect;
const express = require('express');
const launchBrowser = require('./util').launchBrowser;
const takeScreenshot = require('./util').takeScreenshot;

describe('puppeteer', function() {
  let browser, page, server;

  before(async () => {
    server = await startTestServer();
    browser = await launchBrowser();
  });

  after(async () => {
    await browser.close();
    await server.close();
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
    await takeScreenshot(page, 'Test-Puppeteer-Hello-World');
  });
});

// Starts an Express server on a random, unused port. Serves a test page.
const startTestServer = () => {
  const app = express();
  app.get('/', (_, res) => {
    res.send(`
        <html>
        <body>
          <h1>hello</h1>
          <p>world</p>
        </body>
        </html>
    `);
  });
  return new Promise((resolve) => {
    const server = app.listen(0, () => resolve(server));
  });
};
