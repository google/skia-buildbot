import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { CommitNumber } from '../json/index';

describe('commit-detail-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 500, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test. There are now 4 elements: 3 from original demo, 1 new for testing.
    expect(await testBed.page.$$('commit-detail-sk')).to.have.length(3);
  });

  it('should display the correct commit information', async () => {
    const testCommit = {
      hash: 'abcdef1234567890abcdef1234567890abcdef12',
      url: 'https://test.googlesource.com/test/+show/abcdef1234567890abcdef1234567890abcdef12',
      message: 'Test commit message for Puppeteer tests',
      author: 'test-author@google.com',
      ts: 1674823200,
      offset: CommitNumber(123),
      body: 'This is the body of the test commit message.',
    };

    await testBed.page.evaluate((cid) => {
      const ele = document.querySelector<any>('#test-commit-detail');
      if (ele) {
        ele.cid = cid;
      }
    }, testCommit);

    const preText = await testBed.page.$eval(
      '#test-commit-detail pre',
      (el: Element) => (el as HTMLElement).innerText
    );

    // Mock Date.now is '2020-03-22T00:00:00.000Z' in demo.ts,
    // so diffDate(1674823200 * 1000) will be '149w'
    expect(preText).to.include(
      'abcdef12 - test-author@google.com - 149w - Test commit message for Puppeteer tests'
    );
  });

  describe('button functionality', () => {
    const FOUR_DAYS_IN_SECONDS = 4 * 24 * 60 * 60;
    const testCommit = {
      hash: 'explorehash1234567890explorehash1234567890',
      url: 'https://test.googlesource.com/test/+show/explorehash1234567890explorehash1234567890',
      message: 'Explore commit message',
      author: 'explore@google.com',
      ts: 1674823200, // Jan 27 2023 10:00:00 GMT+0000
      offset: CommitNumber(100),
      body: 'Explore body',
    };

    beforeEach(async () => {
      await testBed.page.evaluate((cid) => {
        const ele = document.querySelector<any>('#test-commit-detail');
        if (ele) {
          ele.cid = cid;
          ele.trace_id = ''; // Reset trace_id for each test to avoid interference
        }
      }, testCommit);

      await testBed.page.evaluate(() => {
        window.open = (url?: string | URL) => {
          (window as any).capturedOpenUrl = url;
          return null;
        };
      });
    });

    afterEach(async () => {
      await testBed.page.evaluate(() => {
        delete (window as any).capturedOpenUrl;
      });
    });

    it('should open correct "Explore" link when trace_id is present', async () => {
      await testBed.page.evaluate((traceId) => {
        const ele = document.querySelector<any>('#test-commit-detail');
        if (ele) {
          ele.trace_id = traceId;
        }
      }, 'my-test-trace-id');

      const exploreButton = await testBed.page.$(
        '#test-commit-detail md-outlined-button:nth-child(1)'
      );
      await exploreButton!.click();

      const capturedUrl = await testBed.page.evaluate(() => (window as any).capturedOpenUrl);

      const expectedBegin = testCommit.ts - FOUR_DAYS_IN_SECONDS;
      const expectedEnd = testCommit.ts + FOUR_DAYS_IN_SECONDS;
      const expectedQuery = `begin=${expectedBegin}&end=${expectedEnd}&keys=my-test-trace-id&num_commits=50&request_type=1&xbaroffset=100`;
      expect(capturedUrl).to.equal(`/e/?${expectedQuery}`);
    });

    it('should open correct "Explore" link when trace_id is absent', async () => {
      const exploreButton = await testBed.page.$(
        '#test-commit-detail md-outlined-button:nth-child(1)'
      );
      await exploreButton!.click();

      const capturedUrl = await testBed.page.evaluate(() => (window as any).capturedOpenUrl);
      expect(capturedUrl).to.equal(`/g/e/${testCommit.hash}`);
    });

    it('should open correct "Cluster" link', async () => {
      const clusterButton = await testBed.page.$(
        '#test-commit-detail md-outlined-button:nth-child(2)'
      );
      await clusterButton!.click();

      const capturedUrl = await testBed.page.evaluate(() => (window as any).capturedOpenUrl);
      expect(capturedUrl).to.equal(`/g/c/${testCommit.hash}`);
    });

    it('should open correct "Triage" link', async () => {
      const triageButton = await testBed.page.$(
        '#test-commit-detail md-outlined-button:nth-child(3)'
      );
      await triageButton!.click();

      const capturedUrl = await testBed.page.evaluate(() => (window as any).capturedOpenUrl);
      expect(capturedUrl).to.equal(`/g/t/${testCommit.hash}`);
    });

    it('should open correct "Commit" link', async () => {
      const commitButton = await testBed.page.$(
        '#test-commit-detail md-outlined-button:nth-child(4)'
      );
      await commitButton!.click();

      const capturedUrl = await testBed.page.evaluate(() => (window as any).capturedOpenUrl);
      expect(capturedUrl).to.equal(testCommit.url);
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-sk');
    });

    it('hovers over a button', async () => {
      await testBed.page.hover('md-outlined-button');
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-sk_hover');
    });
  });
});
