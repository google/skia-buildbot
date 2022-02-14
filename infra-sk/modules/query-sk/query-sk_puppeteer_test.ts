import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('query-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 1400 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('query-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'query-sk');
    });
  });
});
