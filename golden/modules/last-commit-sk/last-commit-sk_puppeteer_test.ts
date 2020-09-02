import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('last-commit-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/last-commit-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('last-commit-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('displays commit hash and author', async () => {
      const lastCommitSk = await testBed.page.$('#container');
      await takeScreenshot(lastCommitSk!, 'gold', 'last-commit-sk');
    });
  });
});
