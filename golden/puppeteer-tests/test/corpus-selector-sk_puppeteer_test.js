const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const launchBrowser = require('./util').launchBrowser;
const startDemoPageServer = require('./util').startDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('corpus-selector-sk', function() {
  let browser, baseUrl, stopDemoPageServer;

  before(async () => {
    ({baseUrl, stopDemoPageServer} = await startDemoPageServer());
    browser = await launchBrowser();
  });

  after(async () => {
    await browser.close();
    await stopDemoPageServer();
  });

  let page;

  beforeEach(async () => {
    page = await browser.newPage();

    // Tell the demo page this is a Puppeteer test so it won't fake RPC latency.
    await page.setCookie({url: baseUrl, name: 'puppeteer', value: 'true'});

    // Listen for custom event triggered when component finishes loading.
    const eventPromise =
        await addEventListenersToPuppeteerPage(
            page, ['corpus-selector-sk-loaded']);

    // Page has three corpus selectors so we wait until all of them have loaded.
    const loaded = Promise.all([
        eventPromise('corpus-selector-sk-loaded'),
        eventPromise('corpus-selector-sk-loaded'),
        eventPromise('corpus-selector-sk-loaded'),
    ]);
    await page.goto(`${baseUrl}/dist/corpus-selector-sk.html`);
    await loaded;
  });

  afterEach(async () => {
    await page.close();
  });

  it('should render the demo page', async () => {
    // Basic sanity check.
    expect(await page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('should take a screenshot', async () => {
    await page.setViewport({ width: 1200, height: 1200 });
    await takeScreenshot(page, 'Test-Corpus-Selector-Sk');
  });
});
