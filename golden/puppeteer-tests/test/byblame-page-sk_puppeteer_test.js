const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('byblame-page-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    const eventPromise = await addEventListenersToPuppeteerPage(pp.page, ['end-task']);
    const loaded = eventPromise('end-task'); // Emitted when page is loaded.
    await pp.page.goto(`${pp.baseUrl}/dist/byblame-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('byblame-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await pp.page.setViewport({ width: 1200, height: 7300 });
    await takeScreenshot(pp.page, 'byblame-page-sk');
  });
});
