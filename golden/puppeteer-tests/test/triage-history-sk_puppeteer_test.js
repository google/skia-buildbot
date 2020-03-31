const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('triage-history-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/triage-history-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('triage-history-sk')).to.have.length(2);
  });

  describe('screenshots', async () => {
    it('draws either empty or shows the last history object', async () => {
      await takeScreenshot(pp.page, 'triage-history-sk');
    });
  });
});
