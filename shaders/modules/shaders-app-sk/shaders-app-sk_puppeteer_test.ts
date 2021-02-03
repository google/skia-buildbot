import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('shaders-app-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/shaders-app-sk.html`);
    await testBed.page.setViewport({ width: 1024, height: 512 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('shaders-app-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'shaders', 'shaders-app-sk');
    });
  });
});
