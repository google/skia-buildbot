const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('changelist-controls-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/changelist-controls-sk.html`);
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('changelist-controls-sk')).to.have.length(1);
  });

  it('should take a screenshot', async function() {
    await this.page.setViewport({ width: 1200, height: 250 });
    await takeScreenshot(this.page, 'changelist-controls-sk');
  });
});
