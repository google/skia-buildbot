import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('revision-info-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('revision-info-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'revision-info-sk');
    });

    it('Adds some data to display', async () => {
      await testBed.page.type('#revision_id', '12345');
      await testBed.page.click('#getRevisionInfo');
      await takeScreenshot(
        testBed.page,
        'perf',
        'revision-info-sk data loaded'
      );
    });
  });
});
