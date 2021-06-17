import { expect } from 'chai';
import {inBazel, loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import { BulkTriageSkPO } from './bulk-triage-sk_po';
import { ElementHandle } from 'puppeteer';
import path from "path";

describe('bulk-triage-sk', () => {
  let bulkTriageSk: ElementHandle;
  let bulkTriageSkPO: BulkTriageSkPO;

  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/bulk-triage-sk.html`);

    bulkTriageSk = (await testBed.page.$('#default'))!;
    bulkTriageSkPO = new BulkTriageSkPO(bulkTriageSk);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('bulk-triage-sk')).to.have.length(2); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should be closest by default', async () => {
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_closest');
    });

    it('should be negative', async () => {
      await bulkTriageSkPO.clickUntriagedBtn();
      await testBed.page.click('body'); // Remove focus from button.
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_untriaged');
    });

    it('should be negative', async () => {
      await bulkTriageSkPO.clickNegativeBtn();
      await testBed.page.click('body'); // Remove focus from button.
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_negative');
    });

    it('should be positive', async () => {
      await bulkTriageSkPO.clickPositiveBtn();
      await testBed.page.click('body'); // Remove focus from button.
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_positive');
    });

    it('should be positive, with button focused', async () => {
      await bulkTriageSkPO.clickPositiveBtn();
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_positive-button-focused');
    });

    it('changes views when checkbox clicked', async () => {
      await bulkTriageSkPO.clickTriageAllCheckbox();
      await takeScreenshot(bulkTriageSk, 'gold', 'bulk-triage-sk_triage-all');
    });

    it('shows some extra information for changelists', async () => {
      const bulkTriageSkWithCL = await testBed.page.$('#changelist');
      await takeScreenshot(bulkTriageSkWithCL!, 'gold', 'bulk-triage-sk_changelist');
    });
  });
});
