import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, addEventListenersToPuppeteerPage, takeScreenshot } from '../../../puppeteer-tests/util';

describe('triagelog-page-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
    const loaded = eventPromise('end-task'); // Emitted when page is loaded.
    await testBed.page.goto(`${testBed.baseUrl}/dist/triagelog-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Basic sanity check.
    expect(await testBed.page.$$('triagelog-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 2600 });
    await takeScreenshot(testBed.page, 'gold', 'triagelog-page-sk');
  });
});
