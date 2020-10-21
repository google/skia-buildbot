import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('multi-input-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/multi-input-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('multi-input-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'multi-input-sk');
    });
    it('type to add values', async () => {
      await testBed.page.type('input', 'blahblah');
      await takeScreenshot(testBed.page, 'infra-sk', 'multi-input-sk_typing');
      await testBed.page.click('h2');
      await takeScreenshot(testBed.page, 'infra-sk', 'multi-input-sk_added');
      await testBed.page.type('input', 'another item');
      await testBed.page.click('h2');
      await takeScreenshot(testBed.page, 'infra-sk', 'multi-input-sk_added2');
      await testBed.page.click('close-icon-sk');
      await takeScreenshot(testBed.page, 'infra-sk', 'multi-input-sk_removed');
    });
  });
});
