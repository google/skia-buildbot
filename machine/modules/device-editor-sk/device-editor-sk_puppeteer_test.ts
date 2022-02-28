import {
  inBazel, loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('device-editor-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  describe('screenshots', () => {
    it('shows place holder text when no dimensions are given', async () => {
      await navigateTo('');
      await takeScreenshot(testBed.page, 'machine', 'device-editor-sk');
    });

    it('shows a view when no dimensions given', async () => {
      await navigateTo('#preexisting');
      await takeScreenshot(testBed.page, 'machine', 'device-editor-sk_with_ssh_device');
    });

    it('does not show dimensions if there is no user ip set', async () => {
      await navigateTo('#no_sshuserip');
      await takeScreenshot(testBed.page, 'machine', 'device-editor-sk_normal_machine');
    });
  });

  async function navigateTo(hash: string) {
    await testBed.page.goto(inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/device-editor-sk.html${hash}`);
    await testBed.page.setViewport({ width: 640, height: 480 });
  }
});
