const expect = require('chai').expect;
const path = require('path');
const puppeteer = require('puppeteer');
const startDemoPageServer = require('../util').startDemoPageServer;
const registerCustomEvents = require('../util').registerCustomEvents;
const customEventPromise = require('../util').customEventPromise;
const diffScreenshot = require('../screenshot-diff').diffScreenshot;

describe('triagelog-page-sk', function() {
  const debug = false; // Set to true to test with a headful Chromium and hang.

  this.timeout(5000); // Give the demo page server some extra time to boot.

  //////////////////////////////////////////////////////////////////////////////
  // Set up Puppeteer and the demo page server.                               //
  //////////////////////////////////////////////////////////////////////////////

  let browser, baseUrl, stopDemoPageServer;

  before(async () => {
    ({baseUrl, stopDemoPageServer} = await startDemoPageServer());
    browser = await puppeteer.launch({headless: !debug});
  });

  after(async () => {
    if (debug) {
      await new Promise(() => {}); // Hang and never kill the browser or server.
    }

    await stopDemoPageServer();
    await browser.close();
  });

  //////////////////////////////////////////////////////////////////////////////
  // Create a new page before each test.                                      //
  //////////////////////////////////////////////////////////////////////////////

  let page;

  beforeEach(async () => {
    page = await browser.newPage();
    await page.setViewport({ width: 1200, height: 1600 });

    // Tell the demo page we're in a Puppeteer test so it won't fake RPC
    // latency.
    await page.setCookie({url: baseUrl, name: 'puppeteer', value: 'true'});

    // Listen for the "end-task" event so we know when the page has finished
    // loading.
    await registerCustomEvents(page, ['end-task']);
  });

  afterEach(async () => {
    if (debug) {
      return; // Leave tabs open so we can inspect them manually.
    }

    await page.close();
  });

  //////////////////////////////////////////////////////////////////////////////
  // Tests.                                                                   //
  //////////////////////////////////////////////////////////////////////////////

  it('has the expected number of items', async () => {
    // Go to page and wait for results to load.
    await endTaskEvent(page.goto(demoPageUrl()));

    // Quick sanity check: the logs table has the expected number of entries.
    expect(await page.$eval('triagelog-page-sk tbody',
        el => el.childElementCount))
        .to.equal(20);
  });

  it('details hidden', async () => {
    // Go to page and wait for results to load.
    await endTaskEvent(page.goto(demoPageUrl()));

    await page.screenshot({path: 'details-hidden.png'});

    await diffScreenshot(page, path.join(__dirname, 'details-hidden.png'));
  });

  it('details visible', async () => {
    // Make page taller to fit details.
    await page.setViewport({ width: 1200, height: 4000 });

    // Go to page and wait for results to load.
    await endTaskEvent(page.goto(demoPageUrl()));

    // Show details.
    await endTaskEvent(page.click('checkbox-sk input'));

    await page.screenshot({path: 'details-visible.png'});
  });

  const demoPageUrl = () => `${baseUrl}/dist/triagelog-page-sk.html`;

  const endTaskEvent =
      promise => Promise.all([
          promise,
          customEventPromise('end-task')
      ]);
});
