import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('machine-table-columns-dialog-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 1024 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('machine-table-columns-dialog-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await testBed.page.click('#open');
      await takeScreenshot(testBed.page, 'machine', 'machine-table-columns-dialog-sk');
    });
  });
});
