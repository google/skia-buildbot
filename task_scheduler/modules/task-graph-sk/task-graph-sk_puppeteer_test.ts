import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('task-graph-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 850, height: 400 });
    await testBed.page.evaluate(() => {
      (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = false;
    });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('task-graph-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task_scheduler', 'task-graph-sk');
      // Take a screenshot in dark mode.
      await testBed.page.evaluate(() => {
        (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = true;
      });
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-graph-sk_dark');
    });
  });
});
