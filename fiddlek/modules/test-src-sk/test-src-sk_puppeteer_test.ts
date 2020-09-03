import * as path from 'path';
import { expect } from 'chai';
import { TestSrcSk } from './test-src-sk';
import fetchMock from 'fetch-mock';
import {
  addEventListenersToPuppeteerPage,
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

fetchMock.config.overwriteRoutes = true;

describe('test-src-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/test-src-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('test-src-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'fiddle', 'test-src-sk');
    });
  });
});
