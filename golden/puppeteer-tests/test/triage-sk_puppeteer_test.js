const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('triage-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/triage-sk.html`);
  });

  it('should render the demo page', async () => {
    expect(await pp.page.$$('triage-sk')).to.have.length(1); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should be untriaged by default', async () => {
      const triageSk = await pp.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_untriaged');
    });

    it('should be negative', async () => {
      await pp.page.click('triage-sk button.negative');
      await pp.page.click('body'); // Remove focus from button.
      const triageSk = await pp.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_negative');
    });

    it('should be positive', async () => {
      await pp.page.click('triage-sk button.positive');
      await pp.page.click('body'); // Remove focus from button.
      const triageSk = await pp.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_positive');
    });

    it('should be positive, with button focused', async () => {
      await pp.page.click('triage-sk button.positive');
      const triageSk = await pp.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_positive-button-focused');
    });

    it('should be empty', async () => {
      await pp.page.click('#clear-selection');
      const triageSk = await pp.page.$('triage-sk');
      await takeScreenshot(triageSk, 'triage-sk_empty');
    });
  });
});
