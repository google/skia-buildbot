import * as path from 'path';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('gantt-chart-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/gantt-chart-sk.html`);
    await testBed.page.setViewport({ width: 1000, height: 600 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  describe('screenshots', () => {
    it('simple chart', async () => {
      await testBed.page.click('#simple');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_simple'
      );
    });

    it('simple chart with start and end', async () => {
      await testBed.page.click('#simple-start-end');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_simple-start-end'
      );
    });

    it('simple chart with epochs', async () => {
      await testBed.page.click('#simple-epochs');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_simple-epochs'
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
        'gantt-chart-sk_simple-epochs-dark'
      );
    });

    it('mouse selection', async () => {
      // Display the chart.
      await testBed.page.click('#simple');

      // Move the mouse pointer onto the chart. The screenshot should show a
      // vertical cursor with timestamp.
      await testBed.page.mouse.move(150, 300);
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_mouse-cursor'
      );

      // Move the mouse over a bit. It should snap to the edge of one of the
      // blocks.
      await testBed.page.mouse.move(225, 300);
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_mouse-cursor-snap'
      );

      // Click and drag. The timestamp on the vertical cursor should disappear,
      // and a selection region should appear between the initial click location
      // and the current mouse location.
      await testBed.page.mouse.down();
      await testBed.page.mouse.move(400, 300);
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_mouse-selecting'
      );

      // Release the mouse button and move the pointer away. The selection
      // region should remain, with the mouse cursor off to the side.
      await testBed.page.mouse.move(450, 300);
      await testBed.page.mouse.up();
      await testBed.page.mouse.move(500, 300);
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'gantt-chart-sk_mouse-selected'
      );
    });
  });
});
