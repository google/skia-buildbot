import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed
} from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';

describe('autogrow-textarea-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });
  let textarea: ElementHandle;

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/autogrow-textarea-sk.html`);
    await testBed.page.setViewport({ width: 800, height: 600 });
    textarea = (await testBed.page.$('textarea'))!;
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('autogrow-textarea-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the empty view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk');
    });

    it('shows small amount of text without growth', async () => {
      await textarea.type('A\nfew\nlines don\'t grow the textarea');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_filled');
    });

    it('shows the textarea grows', async () => {
      await textarea.type('A\n\n\nlot\n\n\n\nof lines\n\n\n\n\nhere');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_grow');
    });

    it('shows the textarea shrinks', async () => {
      await textarea.type('Two\nLines');
      await takeScreenshot(testBed.page, 'infra-sk', 'autogrow-textarea-sk_shrink');
    });
  });
});
