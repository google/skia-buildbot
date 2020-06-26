import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('bulk-triage-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/bulk-triage-sk.html`);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('bulk-triage-sk')).to.have.length(2); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should be closest by default', async () => {
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_closest');
    });

    it('should be negative', async () => {
      await testBed.page.click('#default button.untriaged');
      await testBed.page.click('body'); // Remove focus from button.
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_untriaged');
    });

    it('should be negative', async () => {
      await testBed.page.click('#default button.negative');
      await testBed.page.click('body'); // Remove focus from button.
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_negative');
    });

    it('should be positive', async () => {
      await testBed.page.click('#default button.positive');
      await testBed.page.click('body'); // Remove focus from button.
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_positive');
    });

    it('should be positive, with button focused', async () => {
      await testBed.page.click('#default button.positive');
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_positive-button-focused');
    });

    it('changes views when checkbox clicked', async () => {
      await testBed.page.click('#default checkbox-sk.toggle_all');
      const bulkTriageSk = await testBed.page.$('#default');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_triage-all');
    });

    it('shows some extra information for changelists', async () => {
      const bulkTriageSk = await testBed.page.$('#changelist');
      await takeScreenshot(bulkTriageSk!, 'gold', 'bulk-triage-sk_changelist');
    });
  });
});
