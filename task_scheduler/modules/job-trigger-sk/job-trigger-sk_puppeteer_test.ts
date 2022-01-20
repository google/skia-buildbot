import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('job-trigger-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 500, height: 600 });
    await testBed.page.evaluate((_) => {
      (<ThemeChooserSk>(
        document.getElementsByTagName('theme-chooser-sk')[0]
      )).darkmode = false;
    });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('job-trigger-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task_scheduler', 'job-trigger-sk');
      // Take a screenshot in dark mode.
      await testBed.page.evaluate((_) => {
        (<ThemeChooserSk>(
          document.getElementsByTagName('theme-chooser-sk')[0]
        )).darkmode = true;
      });
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-trigger-sk_dark',
      );
    });
    it('deletes job from list', async () => {
      await testBed.page.click('delete-icon-sk');
      await takeScreenshot(
        testBed.page,
        'task_scheduler',
        'job-trigger-sk_deleted',
      );
    });
    it('adds job to list', async () => {
      await testBed.page.click('add-icon-sk');
      await takeScreenshot(
        testBed.page,
        'task_scheduler',
        'job-trigger-sk_added',
      );
    });
    it('triggers jobs', async () => {
      await testBed.page.type('.job_specs_input', 'my-job');
      await testBed.page.type('.commit_input', 'abc123');
      await takeScreenshot(
        testBed.page,
        'task_scheduler',
        'job-trigger-sk_pre-trigger',
      );
      await testBed.page.click('send-icon-sk');
      await takeScreenshot(
        testBed.page,
        'task_scheduler',
        'job-trigger-sk_post-trigger',
      );
    });
  });
});
