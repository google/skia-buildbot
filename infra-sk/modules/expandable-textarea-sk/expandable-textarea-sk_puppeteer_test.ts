import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('expandable-textarea-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/expandable-textarea-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('expandable-textarea-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the closed view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'expandable-textarea-sk_closed');
    });

    it('shows the expanded view', async () => {
      await testBed.page.click('button');
      await takeScreenshot(testBed.page, 'infra-sk', 'expandable-textarea-sk_open');
    });
  });
});
