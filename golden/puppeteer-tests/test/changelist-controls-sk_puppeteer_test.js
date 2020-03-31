const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('changelist-controls-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/changelist-controls-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('changelist-controls-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await pp.page.setViewport({ width: 1200, height: 250 });
    await takeScreenshot(pp.page, 'changelist-controls-sk');
  });
});
