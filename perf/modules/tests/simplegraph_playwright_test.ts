import { expect } from 'chai';
import { loadPlaywrightTestBed, PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';
import { MockServerClient } from './mock_client';
import { SimplegraphPage } from './pages/simplegraph_page';

describe('Simplegraph (/e) workflows', () => {
  let testBed: PlaywrightTestBed;
  let mock: MockServerClient;

  before(async () => {
    testBed = await loadPlaywrightTestBed();
    mock = new MockServerClient(testBed);
  });

  it('plots Chrome jetstream3.crossbench traces on /e and records /_/frame/start', async () => {
    await mock.reset('chrome');
    const simplegraph = new SimplegraphPage(testBed.page, testBed.baseUrl);
    await simplegraph.goto();

    await simplegraph.selectPickerFields({
      benchmark: ['jetstream3.crossbench'],
      bot: ['mac-m3-pro-perf'],
      test: ['Total'],
    });
    const frameReqPromise = testBed.page.waitForRequest(
      (req) => req.url().includes('/_/frame/start') && req.method() === 'POST'
    );
    await simplegraph.clickPlot();
    const frameReq = await frameReqPromise;
    const frameBody = frameReq.postDataJSON() as { queries?: string[] };
    expect(frameBody.queries?.length).to.be.greaterThan(0);
    expect(frameBody.queries?.[0]).to.include('benchmark=jetstream3.crossbench');

    const traceInfo = await simplegraph.getTraceInfo();
    expect(traceInfo.keys.length).to.equal(1);
    expect(traceInfo.keys[0]).to.include('benchmark=jetstream3.crossbench');
    expect(traceInfo.keys[0]).to.include('bot=mac-m3-pro-perf');
    expect(traceInfo.headerCount).to.be.greaterThanOrEqual(20);

    const frames = await mock.recorded<{ queries: string[] }>('/_/frame/start');
    expect(frames.length).to.be.greaterThan(0);
    expect(frames[frames.length - 1].queries[0]).to.include('benchmark=jetstream3.crossbench');

    const shortcuts = await mock.recorded<{ graphs: Array<{ queries: string[] }> }>(
      '/_/shortcut/update'
    );
    expect(shortcuts.length).to.be.greaterThan(0);
    expect(shortcuts[shortcuts.length - 1].graphs[0].queries[0]).to.include(
      'benchmark=jetstream3.crossbench'
    );
  });

  it('queries v8.infra binary_size traces on /e, selects all subtest_1 via checkbox-sk#select-all, and renders d8 and chrome subtests', async () => {
    await mock.reset('v8');
    const simplegraph = new SimplegraphPage(testBed.page, testBed.baseUrl);
    await simplegraph.goto();

    await simplegraph.selectPickerFields({
      benchmark: ['v8.infra'],
      bot: ['linux32'],
      test: ['binary_size'],
      subtest_1: ['d8'],
    });

    const selectedSubtests = await simplegraph.selectAll('subtest_1', true);
    expect(selectedSubtests).to.include.members(['chrome', 'd8']);
    await simplegraph.clickPlot();

    const traceInfo = await simplegraph.getTraceInfo();
    expect(traceInfo.keys.length).to.equal(2);
    expect(traceInfo.keys.some((k) => k.includes('subtest_1=d8'))).to.be.true;
    expect(traceInfo.keys.some((k) => k.includes('subtest_1=chrome'))).to.be.true;
  });

  it('loads /m multigraph and /a regressions pages migrated from initial_loading_test', async () => {
    await mock.reset('chrome');
    await mock.pinExploreV2(false);

    await testBed.page.goto(`${testBed.baseUrl}/m`, { waitUntil: 'networkidle' });
    await testBed.page.locator('explore-multi-sk').waitFor({ state: 'visible', timeout: 10000 });
    await testBed.page
      .locator('explore-multi-sk test-picker-sk')
      .waitFor({ state: 'visible', timeout: 10000 });
    expect(await testBed.page.locator('explore-multi-sk').count()).to.equal(1);

    await testBed.page.goto(`${testBed.baseUrl}/a`, { waitUntil: 'networkidle' });
    await testBed.page.locator('regressions-page-sk').waitFor({ state: 'visible', timeout: 10000 });
    await testBed.page
      .locator('regressions-page-sk select[id^="filter-"]')
      .waitFor({ state: 'visible', timeout: 10000 });
    expect(await testBed.page.locator('regressions-page-sk').count()).to.equal(1);
  });
});
