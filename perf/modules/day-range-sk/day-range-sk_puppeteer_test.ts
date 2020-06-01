import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('day-range-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/day-range-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('day-range-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'day-range-sk');
    });

    it('shows the begin date selector', async () => {
      await testBed.page.click('.begin elix-date-combo-box');
      await takeScreenshot(testBed.page, 'perf', 'day-range-sk_begin-selector');
    });

    it('shows the end date selector', async () => {
      await testBed.page.click('.end elix-date-combo-box');
      await takeScreenshot(testBed.page, 'perf', 'day-range-sk_end-selector');
    });
  });
});
