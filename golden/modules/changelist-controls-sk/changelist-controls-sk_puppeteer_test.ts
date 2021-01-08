import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";

describe('changelist-controls-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/changelist-controls-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('changelist-controls-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    const controls = await testBed.page.$('.search_response');
    await takeScreenshot(controls!, 'gold', 'changelist-controls-sk');
  });
});
