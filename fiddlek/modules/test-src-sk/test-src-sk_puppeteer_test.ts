import * as path from 'path';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

fetchMock.config.overwriteRoutes = true;

describe('test-src-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/test-src-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('test-src-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'fiddle', 'test-src-sk');
    });
  });
});
