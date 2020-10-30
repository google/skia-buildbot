import * as path from 'path';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('task-scheduler-scaffold-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(
      `${testBed.baseUrl}/dist/task-scheduler-scaffold-sk.html`
    );
    await testBed.page.setViewport({ width: 650, height: 400 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'task-scheduler-scaffold-sk'
      );
      // Take a screenshot in dark mode.
      await testBed.page.evaluate((_) => {
        (<ThemeChooserSk>(
          document.getElementsByTagName('theme-chooser-sk')[0]
        )).darkmode = true;
      });
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'task-scheduler-scaffold-sk_dark'
      );
    });
  });
});
