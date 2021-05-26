import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('pathkit-fiddle', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/pathkit-fiddle.html`);
    await testBed.page.setViewport({ width: 1600, height: 800 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('pathkit-fiddle')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'jsfiddle', 'pathkit-fiddle');
    });
  });
});
