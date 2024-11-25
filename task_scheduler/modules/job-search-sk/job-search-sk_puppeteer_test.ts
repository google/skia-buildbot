import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

describe('job-search-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 2200, height: 500 });
    await testBed.page.evaluate(() => {
      (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = false;
    });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('job-search-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('select search terms and search', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-initial');
      await testBed.page.click('select');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-selecting');
      await testBed.page.select('select', 'name');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-selected');
      await testBed.page.type('#name', 'ABCDEF');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-type');
      await testBed.page.click('button.search');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-results');

      // Take a screenshot in dark mode.
      await testBed.page.evaluate(() => {
        (<ThemeChooserSk>document.getElementsByTagName('theme-chooser-sk')[0]).darkmode = true;
      });
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-searching-results-dark');
    });

    it('deletes search terms', async () => {
      await testBed.page.select('select', 'name');
      await testBed.page.type('#name', 'ABCDEF');
      await testBed.page.select('select', 'revision');
      await testBed.page.type('#revision', '9883def4f8661f8eec4ccbae2e34d7fcb14bf65d');
      await testBed.page.click('button.delete');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-deleted-search-term');
    });

    it('cancels a job', async () => {
      await testBed.page.click('button.search');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-cancel-before');
      await testBed.page.click('td > button.cancel');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-cancel-after');
    });

    it('cancels all jobs', async () => {
      await testBed.page.click('button.search');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-cancel-all-before');
      await testBed.page.click('th > button.cancel');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-search-sk-cancel-all-after');
    });
  });
});
