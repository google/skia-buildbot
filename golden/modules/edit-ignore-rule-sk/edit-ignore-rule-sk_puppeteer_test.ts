import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('edit-ignore-rule-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/edit-ignore-rule-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('edit-ignore-rule-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('is a view with nothing selected', async () => {
      const editor = await testBed.page.$('#empty');
      await takeScreenshot(editor!, 'gold', 'edit-ignore-rule-sk');
    });

    it('has all inputs filled out', async () => {
      const editor = await testBed.page.$('#filled');
      await takeScreenshot(editor!, 'gold', 'edit-ignore-rule-sk_with-data');
    });

    it('shows an error when missing data', async () => {
      const editor = await testBed.page.$('#missing');
      await takeScreenshot(editor!, 'gold', 'edit-ignore-rule-sk_missing-data');
    });

    it('shows an error when one or more of custom key/value is not filled out', async () => {
      const editor = await testBed.page.$('#partial_custom_values');
      await takeScreenshot(editor!, 'gold', 'edit-ignore-rule-sk_missing-custom-value');
    });
  });
});
