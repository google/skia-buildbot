import { assert } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('tools-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1800, height: 1800 });
  });

  it('should render the demo page (smoke test)', async () => {
    assert.equal(1, (await testBed.page.$$('tools-sk')).length);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'tools', 'tools-sk-main-page');
    });

    it('shows an individual view', async () => {
      await testBed.page.click('#link-depot_tools');
      await takeScreenshot(testBed.page, 'tools', 'tools-sk-individual');
      await testBed.page.click('#ind-edit-button');
      await takeScreenshot(testBed.page, 'tools', 'tools-sk-edit-individual');
    });

    it('shows the dialog for creating a new tool entry', async () => {
      await testBed.page.click('#new-tool');
      await takeScreenshot(testBed.page, 'tools', 'tools-sk-new');
    });
  });
});
