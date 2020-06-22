import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('autogrow-textarea-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  // TODO: Replace "any" with AutogrowTextareaSk if/when ported to TypeScript.
  let textarea: any;

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/autogrow-textarea-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 700 });
    textarea = await testBed.page.$$('textarea');
  });

  const inputText = (text: string) => {
    textarea.value = text;
    textarea.dispatchEvent(new Event('input', { bubbles: true, cancelable: true }));
  };

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('autogrow-textarea-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the empty view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk');
    });

    it('shows small amount of text without growth', async () => {
      inputText('A\nfew\nlines don\'t grow the textarea');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_filled');
    });
    it('shows the textarea grows', async () => {
      inputText('A\n\n\nlot\n\n\n\nof lines\n\n\n\n\nhere');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_grow');
    });
    it('shows the textarea shrinks', async () => {
      inputText('Two\nLines');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_shrink');
    });
  });
});
