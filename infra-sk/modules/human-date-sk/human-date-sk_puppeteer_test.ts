import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('human-date-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('human-date-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('displays a date', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'human-date-sk');
    });
  });
});
