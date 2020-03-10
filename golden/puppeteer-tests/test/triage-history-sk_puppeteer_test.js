const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('triage-history-sk', () => {
  setUpPuppeteerAndDemoPageServer(); // Sets up this.page and this.baseUrl.

  beforeEach(async function () {
    await this.page.goto(`${this.baseUrl}/dist/triage-history-sk.html`);
  });

  it('should render the demo page', async function () {
    // Smoke test.
    expect(await this.page.$$('triage-history-sk')).to.have.length(2);
  });

  describe('screenshots', async () => {
    it('draws either empty or shows the last history object', async function () {
      await takeScreenshot(this.page, 'triage-history-sk');
    });
  });
});
