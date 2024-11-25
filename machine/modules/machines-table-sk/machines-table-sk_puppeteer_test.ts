import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('machines-table-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 2800, height: 500 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('machines-table-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'machine', 'machines-table-sk');
    });

    it('shows an device-editor-sk dialog', async () => {
      await testBed.page.click('edit-icon-sk.edit_device');
      await takeScreenshot(testBed.page, 'machine', 'machines-table-sk_edit_dialog');
    });

    it('shows a confirm dialog when clearing dimensions', async () => {
      await testBed.page.click('edit-icon-sk.edit_device');
      await testBed.page.click('device-editor-sk .info button.clear');
      await takeScreenshot(testBed.page, 'machine', 'machines-table-sk_confirm_edit_dialog');
    });
  });
});
