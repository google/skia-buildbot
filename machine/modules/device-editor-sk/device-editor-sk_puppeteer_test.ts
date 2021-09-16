import * as path from 'path';
import {
  inBazel, loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('device-editor-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(path.join(__dirname, '..', '..', 'webpack.config.ts'));
  });

  beforeEach(async () => {
    await testBed.page.goto(inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/device-editor-sk.html`);
    await testBed.page.setViewport({ width: 640, height: 480 });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'machine', 'device-editor-sk');
    });
  });
});
