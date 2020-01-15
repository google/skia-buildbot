const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('byblameentry-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/byblameentry-sk.html`);
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('byblameentry-sk')).to.have.length(1);
  });

  it('should take a screenshot', async function() {
    await this.page.setViewport({ width: 1200, height: 700 });
    await takeScreenshot(this.page, 'byblameentry-sk');
  });
});
