import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('favorites-dialog-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('favorites-dialog-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows dialog for new favorite', async () => {
      await testBed.page.click('#newFav');
      await takeScreenshot(testBed.page, 'perf', 'favorites-dialog-sk');
    });

    it('shows dialog for editing an existing favorite', async () => {
      await testBed.page.click('#editFav');
      await takeScreenshot(testBed.page, 'perf', 'favorites-dialog-sk');
    });
  });
});
