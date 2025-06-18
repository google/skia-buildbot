import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('picker-field-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('picker-field-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'picker-field-sk');
    });

    it('was able to activate the combo box', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-focus1');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-field-focus1');
    });

    it('was able to activate the second combo box ', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-focus2');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-field-focus2');
    });

    it('was able to fill the second combo box ', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-fill');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-fill');
    });

    it('was able to open the second combo box ', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-open');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-field-open');
    });

    it('was able to enable the second combo box ', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-enable');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-field-open');
    });

    it('was able to disable the second combo box ', async () => {
      const pickerFieldSk = await testBed.page.$('#demo-disable');
      await takeScreenshot(pickerFieldSk!, 'perf', 'picker-field-open');
    });
  });
});
