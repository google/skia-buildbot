const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('triage-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/triage-sk.html`);
  });

  it('should render the demo page', async function() {
    expect(await this.page.$$('triage-sk')).to.have.length(1);  // Smoke test.
  });

  describe('screenshots', async function() {
    it('should be untriaged by default', async function() {
      const triageSk = await this.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_untriaged');
    });

    it('should be negative', async function() {
      await this.page.click('triage-sk button.negative');
      await this.page.click('body');  // Remove focus from button.
      const triageSk = await this.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_negative');
    });

    it('should be positive', async function() {
      await this.page.click('triage-sk button.positive');
      await this.page.click('body');  // Remove focus from button.
      const triageSk = await this.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_positive');
    });

    it('should be positive, with button focused', async function() {
      await this.page.click('triage-sk button.positive');
      const triageSk = await this.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_positive-button-focused');
    });

    it('should be empty', async function () {
     await this.page.click('#clear-selection');
      const triageSk = await this.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_empty');
    });
  });
});
