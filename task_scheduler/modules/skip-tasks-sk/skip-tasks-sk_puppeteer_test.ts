import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('skip-tasks-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/skip-tasks-sk.html`);
    await testBed.page.setViewport({ width: 550, height: 550 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('skip-tasks-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('starting point', async () => {
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'skip-tasks-sk_start',
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
        'skip-tasks-sk_start-dark',
      );
    });
    it('adds a rule', async () => {
      await testBed.page.click('add-icon-sk');
      await testBed.page.type('#input-name', 'New Rule');
      await testBed.page.type('#input-task-specs input', '.*');
      // TODO(borenet): I would like to use a commit range here, but I was
      // unable to automate the checking of the checkbox and subsequent
      // rendering of the new input field.
      await testBed.page.type('#input-range-start', 'abc123');
      await testBed.page.type(
        '#input-description',
        'This is a detailed description of the rule.',
      );
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'skip-tasks-sk_adding-rule',
      );
      await testBed.page.click('#add-button');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'skip-tasks-sk_added-rule',
      );
    });
  });
});
