import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('fiddle-embed', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/fiddle-embed.html`);
    await testBed.page.setViewport({ width: 400, height: 1200 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('fiddle-embed')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await testBed.page.click('#mode_start');
      await testBed.page.waitForSelector('#complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'fiddle-embed');
    });
  });
});
