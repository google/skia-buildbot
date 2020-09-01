import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('{{.ElementName}}', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadWebpack();
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/{{.ElementName}}.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('{{.ElementName}}')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 600 });
    await takeScreenshot(
      testBed.page,
      'change-me-to-the-app-name',
      '{{.ElementName}}'
    );
  });
});
