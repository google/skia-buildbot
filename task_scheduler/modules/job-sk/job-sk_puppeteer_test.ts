import * as path from 'path';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('job-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/job-sk.html`);
    await testBed.page.setViewport({ width: 835, height: 1110 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-sk');
      // Take a screenshot in dark mode.
      await testBed.page.evaluate((_) => {
        (<ThemeChooserSk>(
          document.getElementsByTagName('theme-chooser-sk')[0]
        )).darkmode = true;
      });
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-sk_dark');
    });
    it('cancels jobs', async () => {
      await testBed.page.click('#cancelButton');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-sk-canceled');
    });
  });
});
