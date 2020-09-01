import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('example-control-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadWebpack();
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/example-control-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('example-control-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 600 });
    await takeScreenshot(
      testBed.page,
      'change-me-to-the-app-name',
      'example-control-sk'
    );
  });
});
