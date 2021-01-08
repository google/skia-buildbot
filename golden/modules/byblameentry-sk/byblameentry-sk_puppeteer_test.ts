import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";

describe('byblameentry-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/byblameentry-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('byblameentry-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    const byBlameEntry = await testBed.page.$('byblameentry-sk');
    await takeScreenshot(byBlameEntry!, 'gold', 'byblameentry-sk');
  });
});
