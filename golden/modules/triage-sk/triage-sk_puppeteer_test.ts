import { expect } from 'chai';
import {inBazel, loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";
import {TriageSkPO} from './triage-sk_po';
import {ElementHandle} from 'puppeteer';

describe('triage-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/triage-sk.html`);
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
    })

    it('should be untriaged by default', async () => {
      await takeScreenshot(triageSk, 'gold', 'triage-sk_untriaged');
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

    it('should be empty', async () => {
      await testBed.page.click('#clear-selection');
      await takeScreenshot(triageSk, 'gold', 'triage-sk_empty');
    });
  });
});
