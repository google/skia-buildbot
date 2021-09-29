import * as path from 'path';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('task-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/task-sk.html`);
    await testBed.page.setViewport({ width: 1429, height: 836 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-sk');
      // Take a screenshot in dark mode.
      await testBed.page.evaluate((_) => {
        (<ThemeChooserSk>(
          document.getElementsByTagName('theme-chooser-sk')[0]
        )).darkmode = true;
      });
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-sk_dark');
    });
  });
});
