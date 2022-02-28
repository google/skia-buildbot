import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('fiddle-embed-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 1200 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('fiddle-embed-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await testBed.page.click('#mode_start');
      await testBed.page.waitForSelector('#complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'fiddle-embed-sk');
    });
  });
});
