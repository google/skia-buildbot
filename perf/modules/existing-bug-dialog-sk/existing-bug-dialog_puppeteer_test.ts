import { expect } from 'chai';
import { ElementHandle } from 'puppeteer';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ExistingBugDialogSkPO } from './existing-bug-dialog-sk_po';
import { anomalies, bugIdTitleMap } from './test_data';

describe('existing-bug-dialog-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  let existingBugDialogSk: ElementHandle;
  let existingBugDialogSkPO: ExistingBugDialogSkPO;

  beforeEach(async () => {
    await testBed.page.setViewport({ width: 800, height: 600 });
    await testBed.page.goto(testBed.baseUrl);
    existingBugDialogSk = (await testBed.page.$('existing-bug-dialog-sk'))!;
    existingBugDialogSkPO = new ExistingBugDialogSkPO(existingBugDialogSk);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('existing-bug-dialog-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await existingBugDialogSk.evaluate((el: any) => {
      el.open();
    });
    await takeScreenshot(testBed.page, 'perf', 'existing-bug-dialog-sk');
  });

  it('should show associated bugs', async () => {
    await existingBugDialogSk.evaluate((el: any, anomalies: any) => {
      el.anomalies = anomalies;
      el.traceNames = ['trace1', 'trace2'];
      el.fetch_associated_bugs();
    }, anomalies);

    await existingBugDialogSk.evaluate(async (el: any, bugIdTitleMap: any) => {
      el.bugIdTitleMap = bugIdTitleMap;
      await el.updateComplete;
    }, bugIdTitleMap);

    await existingBugDialogSk.evaluate((el: any) => {
      el.open();
    });
    await takeScreenshot(testBed.page, 'perf', 'existing-bug-dialog-sk-with-associated-bugs');
  });

  it('should submit a bug', async () => {
    await existingBugDialogSk.evaluate((el: any) => {
      el.open();
    });
    await existingBugDialogSkPO.setBugId('12345');
    await existingBugDialogSkPO.clickSubmitBtn();
    // We can't assert that a new tab was opened, but we can check that the dialog closes.
    expect(await existingBugDialogSkPO.isDialogOpen()).to.equal(true);
  });

  it('should close dialog', async () => {
    await existingBugDialogSk.evaluate((el: any) => {
      el.open();
    });
    await existingBugDialogSkPO.setBugId('12345');
    await existingBugDialogSkPO.clickCloseBtn();
    // We can't assert that a new tab was opened, but we can check that the dialog closes.
    expect(await existingBugDialogSkPO.isDialogOpen()).to.equal(false);
  });
});
