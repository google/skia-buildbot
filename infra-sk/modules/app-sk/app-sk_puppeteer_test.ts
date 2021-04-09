import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel, loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('app-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/app-sk.html`);
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
