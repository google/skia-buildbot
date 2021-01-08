import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed
} from '../../../puppeteer-tests/util';

describe('email-chooser-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/static/email-chooser-sk.html`);
    await testBed.page.setViewport({ width: 300, height: 600 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('email-chooser-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'am', 'email-chooser-sk');
    });

    it('shows that a non-owner is selected', async () => {
      await testBed.page.click('email-chooser-sk option:nth-child(2)');
      await takeScreenshot(testBed.page, 'am', 'email-chooser-sk_non-owner-selected');
    });

    it('shows that the owner is selected', async () => {
      await testBed.page.click('email-chooser-sk option:nth-child(3)');
      await takeScreenshot(testBed.page, 'am', 'email-chooser-sk_owner-selected');
    });
  });
});
