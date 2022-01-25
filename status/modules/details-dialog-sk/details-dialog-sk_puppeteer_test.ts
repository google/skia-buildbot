import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('details-dialog-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('details-dialog-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'details-dialog-sk');
    });

    it('shows task dialog', async () => {
      await testBed.page.click('#taskButton');
      await takeScreenshot(testBed.page, 'status', 'details-dialog-sk_task');
    });

    it('shows task dialog with taskdriver', async () => {
      await testBed.page.click('#taskDriverButton');
      await testBed.page.waitForSelector('task-driver-sk');
      await takeScreenshot(testBed.page, 'status', 'details-dialog-sk_task_driver');
    });

    it('shows taskSpec dialog', async () => {
      await testBed.page.click('#taskSpecButton');
      await takeScreenshot(testBed.page, 'status', 'details-dialog-sk_taskspec');
    });

    it('shows commit dialog', async () => {
      await testBed.page.click('#commitButton');
      await takeScreenshot(testBed.page, 'status', 'details-dialog-sk_commit');
    });
  });
});
