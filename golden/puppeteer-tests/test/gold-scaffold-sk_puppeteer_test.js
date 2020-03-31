const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('gold-scaffold-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/gold-scaffold-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('gold-scaffold-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await pp.page.setViewport({ width: 1200, height: 600 });
    await takeScreenshot(pp.page, 'gold-scaffold-sk');
  });
});
