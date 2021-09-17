import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel, loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('machine-server-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(path.join(__dirname, '..', '..', 'webpack.config.ts'));
  });

  beforeEach(async () => {
    await testBed.page.goto(inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/machine-server-sk.html`);
    await testBed.page.setViewport({ width: 2000, height: 500 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('machine-server-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'machine', 'machine-server-sk');
    });

    it('shows an device-editor-sk dialog', async () => {
      await testBed.page.click('edit-icon-sk.edit_device');
      await takeScreenshot(testBed.page, 'machine', 'machine-server-sk_edit_dialog');
    });

    it('shows a confirm dialog when clearing dimensions', async () => {
      await testBed.page.click('edit-icon-sk.edit_device');
      await testBed.page.click('device-editor-sk .info button.clear');
      await takeScreenshot(testBed.page, 'machine', 'machine-server-sk_confirm_edit_dialog');
    });
  });
});
