import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('app-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1000, height: 1000 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('app-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('renders correctly', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'app-sk');
    });
  });
});
