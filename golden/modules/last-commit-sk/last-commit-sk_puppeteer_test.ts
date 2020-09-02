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

  it('takes a screenshot', async () => {
    const lastCommitSk = await testBed.page.$('#container');
    await takeScreenshot(lastCommitSk!, 'gold', 'last-commit-sk');
  });

  it('has the correct link and text', async() => {
    const lastCommit = await testBed.page.$('#container a');
    const lastCommitHref = await lastCommit?.evaluate((link : Element) => {
      return (link as HTMLAnchorElement).href;
    });
    expect(lastCommitHref).to.equal(
        'https://github.com/flutter/flutter/commit/a8281e31afa9dddfa0764f59128c3a2360c48f49');

    const lastCommitText = await lastCommit?.evaluate((link : Element) => {
      return (link as HTMLAnchorElement).innerText;
    });
    expect(lastCommitText).to.equal('Last Commit: a8281e3 - Foxtrot Delta');
  });
});
