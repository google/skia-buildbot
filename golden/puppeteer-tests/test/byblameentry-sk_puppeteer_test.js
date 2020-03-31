const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('byblameentry-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/byblameentry-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('byblameentry-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await pp.page.setViewport({ width: 600, height: 800 });
    const byBlameEntry = await pp.page.$('byblameentry-sk');
    await takeScreenshot(byBlameEntry, 'byblameentry-sk');
  });
});
