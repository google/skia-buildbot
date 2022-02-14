import * as path from 'path';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('task-driver-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
  });

  describe('screenshots', () => {
    it('shows the failures expanded by default', async () => {
      await testBed.page.setViewport({ width: 800, height: 1000 });
      await takeScreenshot(testBed.page, 'infra-sk', 'task-driver-sk_default');
    });

    it('expands children on a click', async () => {
      await testBed.page.setViewport({ width: 800, height: 1800 });
      await testBed.page.click('#button_children_f7bc5c4f-1bf5-493b-b8e4-fa288df1d949');
      await takeScreenshot(testBed.page, 'infra-sk', 'task-driver-sk_expanded');
    });
  });
});
