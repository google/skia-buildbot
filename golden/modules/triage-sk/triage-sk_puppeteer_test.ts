import { expect } from 'chai';
import { ElementHandle } from 'puppeteer';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { TriageSkPO } from './triage-sk_po';

describe('triage-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('triage-sk')).to.have.length(1); // Smoke test.
  });

  describe('screenshots', async () => {
    let triageSk: ElementHandle;
    let triageSkPO: TriageSkPO;

    beforeEach(async () => {
      triageSk = (await testBed.page.$('triage-sk'))!;
      triageSkPO = new TriageSkPO(triageSk);
    });

    it('should be untriaged by default', async () => {
      await takeScreenshot(triageSk, 'gold', 'triage-sk_untriaged');
    });

    it('should be read only', async () => {
      await testBed.page.click('#read-only-checkbox');
      await takeScreenshot(triageSk, 'gold', 'triage-sk_read-only');
    });

    it('should be negative', async () => {
      await triageSkPO.clickButton('negative');
      await testBed.page.click('body'); // Remove focus from button.
      await takeScreenshot(triageSk, 'gold', 'triage-sk_negative');
    });

    it('should be positive', async () => {
      await triageSkPO.clickButton('positive');
      await testBed.page.click('body'); // Remove focus from button.
      await takeScreenshot(triageSk, 'gold', 'triage-sk_positive');
    });

    it('should be positive, with button focused', async () => {
      await triageSkPO.clickButton('positive');
      await takeScreenshot(triageSk, 'gold', 'triage-sk_positive-button-focused');
    });
  });
});
