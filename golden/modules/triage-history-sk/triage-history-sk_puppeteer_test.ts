import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";

describe('triage-history-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/triage-history-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('triage-history-sk')).to.have.length(2);
  });

  describe('screenshots', async () => {
    // We choose not to draw these two cases separately, since a blank image in Gold would likely
    // be confusing, despite being correct. Showing the headers adds a bit of documentation to the
    // images.
    it('draws either empty or shows the last history object', async () => {
      await testBed.page.setViewport({ width: 400, height: 200 });
      await takeScreenshot(testBed.page, 'gold', 'triage-history-sk');
    });
  });
});
