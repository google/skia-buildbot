import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('theme-chooser-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1500, height: 2000 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('theme-chooser-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default theme', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'theme-chooser-sk_default');
    });

    it('shows the light theme', async () => {
      await testBed.page.click('theme-chooser-sk');
      await takeScreenshot(testBed.page, 'infra-sk', 'theme-chooser-sk_light');
    });
  });
});
