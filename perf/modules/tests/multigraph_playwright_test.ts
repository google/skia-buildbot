import { expect } from 'chai';
import { loadPlaywrightTestBed, PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';
import { MockServerClient } from './mock_client';
import { MultigraphPage } from './pages/multigraph_page';

describe('Multigraph (/m) workflows', () => {
  let testBed: PlaywrightTestBed;
  let mock: MockServerClient;

  before(async () => {
    testBed = await loadPlaywrightTestBed();
    mock = new MockServerClient(testBed);
  });

  it('plots Chrome traces, selects all bots via checkbox-sk#select-all, splits by bot, and unsplits', async () => {
    await mock.reset('chrome');
    const multigraph = new MultigraphPage(testBed.page, testBed.baseUrl);
    await multigraph.goto();

    await multigraph.selectPickerFields({
      benchmark: ['jetstream3.crossbench'],
      bot: ['mac-m3-pro-perf'],
      test: ['Total'],
      subtest_1: ['JetStream2'],
      subtest_2: ['Total'],
    });
    await multigraph.clickPlot();

    await multigraph.waitForGraphCount(1);
    const singleBotTraces = await multigraph.getGraphTraceKeys(0);
    expect(singleBotTraces.length).to.equal(1);

    const selectedBots = await multigraph.selectAll('bot', true);
    expect(selectedBots).to.include.members(['linux-r350-perf', 'mac-m3-pro-perf']);

    const mergedTraces = await multigraph.getGraphTraceKeys(0);
    expect(mergedTraces.length).to.equal(2);

    await multigraph.setSplitBy('bot', true);
    await multigraph.waitForGraphCount(2);
    expect(testBed.page.url()).to.include('splitByKeys=bot');

    const g0Traces = await multigraph.getGraphTraceKeys(0);
    const g1Traces = await multigraph.getGraphTraceKeys(1);
    expect(g0Traces.length).to.equal(1);
    expect(g1Traces.length).to.equal(1);
    expect(g0Traces[0]).to.not.equal(g1Traces[0]);

    await multigraph.setSplitBy('bot', false);
    await multigraph.waitForGraphCount(1);
  });

  it('plots V8 JetStream2 and JetStream3 traces and selects all subtest_1 values via checkbox-sk#select-all', async () => {
    await mock.reset('v8');
    const multigraph = new MultigraphPage(testBed.page, testBed.baseUrl);
    await multigraph.goto();

    await multigraph.selectPickerFields({
      benchmark: ['v8'],
      bot: ['MacM4'],
      test: ['JetStream2', 'JetStream3'],
      subtest_1: ['default'],
      subtest_2: ['Total'],
    });
    await multigraph.clickPlot();

    await multigraph.waitForGraphCount(1);
    const initialTraces = await multigraph.getGraphTraceKeys(0);
    expect(initialTraces.length).to.equal(2);
    expect(initialTraces.every((k) => k.includes('subtest_1=default'))).to.be.true;

    const selectedSubtests = await multigraph.selectAll('subtest_1', true);
    expect(selectedSubtests).to.include.members(['default', 'future']);

    const allSubtestTraces = await multigraph.getGraphTraceKeys(0);
    expect(allSubtestTraces.length).to.equal(4);
    expect(allSubtestTraces.some((k) => k.includes('subtest_1=default'))).to.be.true;
    expect(allSubtestTraces.some((k) => k.includes('subtest_1=future'))).to.be.true;
    expect(allSubtestTraces.some((k) => k.includes('test=JetStream2'))).to.be.true;
    expect(allSubtestTraces.some((k) => k.includes('test=JetStream3'))).to.be.true;
  });

  it('synchronizes split summary range selections and resizes, updates traces, and restores state across reload', async () => {
    await mock.reset('chrome');
    const multigraph = new MultigraphPage(testBed.page, testBed.baseUrl);
    await multigraph.goto();

    await multigraph.selectPickerFields({
      benchmark: ['jetstream3.crossbench'],
      bot: ['mac-m3-pro-perf', 'linux-r350-perf'],
      test: ['Total'],
    });
    await multigraph.clickPlot();
    await multigraph.waitForGraphCount(1);

    await multigraph.setSplitBy('bot', true);
    await multigraph.waitForGraphCount(2);

    const selectAndVerifyRangeSync = async (startIdx: number, endIdx: number) => {
      const range = await multigraph.selectRangeOnSplitGraph(0, startIdx, endIdx);
      const synced = await multigraph.getVisibleSplitGraphRanges();
      expect(synced.length).to.equal(2);
      expect(synced[1].summaryBegin).to.equal(range.beginOffset);
      expect(synced[1].summaryEnd).to.equal(range.endOffset);
      expect(synced[1].stateBegin).to.equal(range.beginTimestamp);
      expect(synced[1].stateEnd).to.equal(range.endTimestamp);
      expect(testBed.page.url()).to.include(`begin=${range.beginTimestamp}`);
      expect(testBed.page.url()).to.include(`end=${range.endTimestamp}`);
      return range;
    };

    await selectAndVerifyRangeSync(2, 10);
    const resizedRange = await selectAndVerifyRangeSync(5, 15);

    const initialTraceCount = (await multigraph.getGraphTraceKeys(0)).length;
    await multigraph.selectAll('test', true);
    await multigraph.waitForGraphCount(2);
    const expandedTraces = await multigraph.getGraphTraceKeys(0);
    expect(expandedTraces.length).to.be.greaterThan(initialTraceCount);
    expect(expandedTraces.some((k) => k.includes('test=Score'))).to.be.true;

    await multigraph.removePickerValue('test', 'Score');
    await multigraph.waitForGraphCount(2);
    const remainingTraces = await multigraph.getGraphTraceKeys(0);
    expect(remainingTraces.length).to.equal(initialTraceCount);
    expect(remainingTraces.every((k) => k.includes('test=Total'))).to.be.true;

    const shortcuts = await mock.recorded('/_/shortcut/update');
    expect(shortcuts.length).to.be.greaterThanOrEqual(1);

    await testBed.page.waitForFunction(() => window.location.search.includes('shortcut='));
    const urlBeforeReload = testBed.page.url();
    expect(urlBeforeReload).to.include('shortcut=');
    expect(urlBeforeReload).to.include('splitByKeys=bot');

    await testBed.page.reload({ waitUntil: 'networkidle' });
    await multigraph.waitForGraphCount(2);

    const restoredRanges = await multigraph.getVisibleSplitGraphRanges();
    expect(restoredRanges.length).to.equal(2);
    expect(restoredRanges[0].stateBegin).to.equal(resizedRange.beginTimestamp);
    expect(restoredRanges[1].stateBegin).to.equal(resizedRange.beginTimestamp);
    expect(restoredRanges[0].stateEnd).to.equal(resizedRange.endTimestamp);
    expect(restoredRanges[1].stateEnd).to.equal(resizedRange.endTimestamp);
  });

  it('loads heavy-shortcut-1 with splitByKeys=bot and opens chart-tooltip-sk on the linux-r350-perf anomaly', async () => {
    await mock.reset('chrome');
    const multigraph = new MultigraphPage(testBed.page, testBed.baseUrl);
    await multigraph.goto('shortcut=heavy-shortcut-1&splitByKeys=bot');

    await multigraph.waitForGraphCount(2);

    const g0Traces = await multigraph.getGraphTraceKeys(0);
    const linuxGraphIndex = g0Traces.some((k) => k.includes('bot=linux-r350-perf')) ? 0 : 1;

    const tooltipInfo = await multigraph.openAnomalyTooltipOnGraph(linuxGraphIndex, 67132);
    expect(tooltipInfo.anomalyId).to.equal('456');
    expect(tooltipInfo.anomalyType).to.equal('Regression');
    expect(tooltipInfo.commitPosition).to.equal(67132);
    expect(tooltipInfo.traceName).to.include('bot=linux-r350-perf');
    expect(tooltipInfo.tooltipText).to.include('Regression');
    expect(tooltipInfo.tooltipText).to.include('82');
  });
});
