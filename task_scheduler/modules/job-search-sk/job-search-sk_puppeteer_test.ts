import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('job-search-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/job-search-sk.html`);
    await testBed.page.setViewport({ width: 2200, height: 500 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('job-search-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('select search terms and search', async () => {
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-searching-initial'
      );
      await testBed.page.click('select');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-searching-selecting'
      );
      await testBed.page.select('select', 'name');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-searching-selected'
      );
      await testBed.page.type('#name', 'ABCDEF');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-searching-type'
      );
      await testBed.page.click('button.search');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-searching-results'
      );
    });

    it('deletes search terms', async () => {
      await testBed.page.select('select', 'name');
      await testBed.page.type('#name', 'ABCDEF');
      await testBed.page.select('select', 'revision');
      await testBed.page.type(
        '#revision',
        '9883def4f8661f8eec4ccbae2e34d7fcb14bf65d'
      );
      await testBed.page.click('button.delete');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-deleted-search-term'
      );
    });

    it('cancels a job', async () => {
      await testBed.page.click('button.search');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-cancel-before'
      );
      await testBed.page.click('td > button.cancel');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-cancel-after'
      );
    });

    it('cancels all jobs', async () => {
      await testBed.page.click('button.search');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-cancel-all-before'
      );
      await testBed.page.click('th > button.cancel');
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'job-search-sk-cancel-all-after'
      );
    });
  });
});
