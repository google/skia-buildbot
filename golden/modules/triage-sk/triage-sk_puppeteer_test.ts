import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('triage-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/triage-sk.html`);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('triage-sk')).to.have.length(1); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should be untriaged by default', async () => {
      const triageSk = await testBed.page.$('triage-sk');
      await takeScreenshot(triageSk!, 'gold', 'triage-sk_untriaged');
    });

    it('should be negative', async () => {
      await testBed.page.click('triage-sk button.negative');
      await testBed.page.click('body'); // Remove focus from button.
      const triageSk = await testBed.page.$('triage-sk');
      await takeScreenshot(triageSk!, 'gold', 'triage-sk_negative');
    });

    it('should be positive', async () => {
      await testBed.page.click('triage-sk button.positive');
      await testBed.page.click('body'); // Remove focus from button.
      const triageSk = await testBed.page.$('triage-sk');
      await takeScreenshot(triageSk!, 'gold', 'triage-sk_positive');
    });

    it('should be positive, with button focused', async () => {
      await testBed.page.click('triage-sk button.positive');
      const triageSk = await testBed.page.$('triage-sk');
      await takeScreenshot(triageSk!, 'gold', 'triage-sk_positive-button-focused');
    });

    it('should be empty', async () => {
      await testBed.page.click('#clear-selection');
      const triageSk = await testBed.page.$('triage-sk');
      await takeScreenshot(triageSk!, 'gold', 'triage-sk_empty');
    });
  });
});
