const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('byblameentry-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/byblameentry-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('byblameentry-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 600, height: 800 });
    const byBlameEntry = await testBed.page.$('byblameentry-sk');
    await takeScreenshot(byBlameEntry, 'byblameentry-sk');
  });
});
