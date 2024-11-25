import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('task-scheduler-scaffold-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 650, height: 400 });
    await testBed.page.evaluate(() => {
      (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = false;
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-scheduler-scaffold-sk');
      // Take a screenshot in dark mode.
      await testBed.page.evaluate(() => {
        (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = true;
      });
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-scheduler-scaffold-sk_dark');
    });
  });
});
