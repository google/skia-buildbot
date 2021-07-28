import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';

describe('byblameentry-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
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
