import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('fiddle-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts'),
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/fiddle-sk.html`, { waitUntil: 'networkidle0' });
    await testBed.page.setViewport({ width: 1024, height: 1400 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('fiddle-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await testBed.page.click('#mode_start');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_start');
    });
    it('displays clickable errors', async () => {
      await testBed.page.click('#mode_after_run_errors');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_after_run_errors');
    });
    it('displays images on a successful run', async () => {
      await testBed.page.click('#mode_after_run_success');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_after_run_success');
    });
    it('displays videos on a successful run', async () => {
      await testBed.page.click('#mode_animation');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_animation');
    });
    it('has a reduced UI when in basic/embedded mode', async () => {
      await testBed.page.click('#mode_basic');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_basic');
    });
    it('correctly displays text only results', async () => {
      await testBed.page.click('#mode_text');
      await testBed.page.waitForSelector('#mode_complete pre');
      await takeScreenshot(testBed.page, 'fiddle', 'mode_text');
    });
  });
});
