import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { CommitRangeSkPO } from './commit-range-sk_po';
import { ElementHandle } from 'puppeteer';

describe('commit-range-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let commitRangeSk: ElementHandle;
  let commitRangeSkPO: CommitRangeSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    commitRangeSk = (await testBed.page.$('commit-range-sk'))!;
    commitRangeSkPO = new CommitRangeSkPO(commitRangeSk);
  });

  it('should have the correct link', async () => {
    const expectedHref =
      'http://example.com/range/3b8de1058a896b613b451db1b6e2b28d58f64a4a' +
      '/9039c60688c9511f9a553cd2443e412f68b5a107';
    const actualHref = await commitRangeSkPO.getHref();
    expect(actualHref).to.equal(expectedHref);
  });
});
