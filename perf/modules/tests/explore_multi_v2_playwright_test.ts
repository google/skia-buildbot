import { expect } from 'chai';
import { loadPlaywrightTestBed, PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';
import { MockServerClient } from './mock_client';
import { ExploreV2Page } from './pages/explore_v2_page';

describe('Explore Multi V2 (/e2) workflows', () => {
  let testBed: PlaywrightTestBed;
  let mock: MockServerClient;

  before(async () => {
    testBed = await loadPlaywrightTestBed();
    mock = new MockServerClient(testBed);
  });

  it('filters V8 traces, toggles pill dropdowns, Ctrl+Clicks pills, and syncs split chart viewports', async () => {
    await mock.reset('v8');
    const v2 = new ExploreV2Page(testBed.page, testBed.baseUrl);
    await v2.goto();

    await v2.typeInSearchInput(0, 'v8');
    const suggestionsCount = await v2.queryBars
      .nth(0)
      .locator('.suggestions-dropdown .suggestion-item')
      .count();
    expect(suggestionsCount).to.be.greaterThan(0);

    await v2.addQueryFilters(0, {
      benchmark: 'v8',
      bot: 'MacM4',
      test: 'JetStream2',
      subtest_1: 'default',
      subtest_2: 'Total',
    });
    await v2.waitForChartCount(1);

    await v2.togglePillOption(0, 'test', 'JetStream3');
    await v2.togglePillOption(0, 'subtest_1', 'future');
    expect((await v2.getSeriesIds(0)).length).to.equal(4);

    await v2.togglePillOption(0, 'subtest_1', 'future');
    expect((await v2.getSeriesIds(0)).length).to.equal(2);

    expect(await v2.ctrlClickPill(0, 0)).to.deep.equal([0]);
    expect(await v2.ctrlClickPill(0, 1)).to.deep.equal([0, 1]);

    await v2.toggleSplitBy('test');
    await v2.waitForChartCount(2);

    const splitPillText = await v2.root.locator('.config-pill').first().innerText();
    expect(splitPillText).to.include('Split by:');
    expect(splitPillText).to.include('test');

    await v2.selectViewportRange(67124, 67134);
    const viewports = await v2.getChartViewports();
    expect(viewports).to.deep.equal([
      { minCommit: 67124, maxCommit: 67134 },
      { minCommit: 67124, maxCommit: 67134 },
    ]);
    expect(testBed.page.url()).to.include('begin=1584849600');
    expect(testBed.page.url()).to.include('end=1584885600');

    await v2.toggleSplitBy('test');
    await v2.waitForChartCount(1);

    await v2.setToolbarControl('splitAll', true);
    await v2.waitForChartCount(2);
    await v2.setToolbarControl('splitAll', false);
    await v2.waitForChartCount(1);
  });

  it('toggles Diff Base, plots a second query bar side-by-side, and chains fill() -> ave() -> norm()', async () => {
    await mock.reset('chrome');
    const v2 = new ExploreV2Page(testBed.page, testBed.baseUrl);
    await v2.goto();

    await v2.addQueryFilter(0, 'benchmark', 'jetstream3.crossbench');
    await v2.setQueryValues(0, 'bot', ['mac-m3-pro-perf', 'linux-r350-perf']);
    await v2.addQueryFilter(0, 'test', 'Total');
    await v2.waitForChartCount(1);

    await v2.toggleDiffBase('bot', 'linux-r350-perf');
    expect(await v2.root.locator('.config-pill.diff-base').isVisible()).to.be.true;

    await v2.toggleDiffBase('bot', 'linux-r350-perf');
    expect(await v2.root.locator('.config-pill.diff-base').count()).to.equal(0);

    await v2.addQueryRow();
    expect(await v2.queryBars.count()).to.equal(2);

    await v2.addQueryFilters(1, {
      benchmark: 'v8.infra',
      bot: 'MacM4',
      test: 'binary_size',
    });

    const combinedSeriesIds = await v2.getSeriesIds(0);
    expect(combinedSeriesIds.some((id) => id.includes('benchmark=jetstream3.crossbench'))).to.be
      .true;
    expect(combinedSeriesIds.some((id) => id.includes('benchmark=v8.infra'))).to.be.true;

    await v2.removeQueryRow(1);
    expect(await v2.queryBars.count()).to.equal(1);

    await v2.addFormulaSteps(0, ['fill', 'ave', 'norm']);

    const stepPills = await v2.queryBars.nth(0).locator('.qb-pipeline-step').allInnerTexts();
    expect(stepPills.some((t) => t.includes('fill()'))).to.be.true;
    expect(stepPills.some((t) => t.includes('ave()'))).to.be.true;
    expect(stepPills.some((t) => t.includes('norm()'))).to.be.true;

    const seriesIds = await v2.getSeriesIds(0);
    expect(seriesIds.length).to.equal(1);
    expect(seriesIds[0]).to.include('norm(ave(fill(filter(');
  });

  it('zooms and resets viewport bounds, persists toolbar toggles across reload, and runs the interactive tour', async () => {
    await mock.reset('chrome');
    const v2 = new ExploreV2Page(testBed.page, testBed.baseUrl);
    await v2.goto();

    await v2.addQueryFilters(0, {
      benchmark: 'jetstream3.crossbench',
      bot: 'mac-m3-pro-perf',
      test: 'Total',
    });
    await v2.waitForChartCount(1);

    expect((await v2.getChartViewports())[0]).to.deep.equal({ minCommit: 67120, maxCommit: 67139 });
    await v2.selectViewportRange(67125, 67133);
    expect((await v2.getChartViewports())[0]).to.deep.equal({ minCommit: 67125, maxCommit: 67133 });
    await v2.resetZoom();
    expect((await v2.getChartViewports())[0]).to.deep.equal({ minCommit: 67120, maxCommit: 67139 });

    expect(await v2.getToolbarState()).to.deep.equal({
      dateMode: false,
      evenXAxisSpacing: false,
      showZero: false,
      hoverMode: 'original',
      showDots: true,
      showRegressions: true,
      normalizeCentre: 'none',
    });

    await v2.setToolbarControls({
      dateMode: true,
      evenXAxisSpacing: true,
      showZero: true,
      smooth: true,
      hoverMode: 'both',
      showDots: false,
      showRegressions: false,
      normalizeCentre: 'first',
    });

    const urlBeforeReload = testBed.page.url();
    expect(urlBeforeReload).to.include('dateMode=true');
    expect(urlBeforeReload).to.include('evenXAxisSpacing=true');
    expect(urlBeforeReload).to.include('centre=first');
    expect(urlBeforeReload).to.include('dots=false');
    expect(urlBeforeReload).to.include('shortcut=');

    await testBed.page.reload({ waitUntil: 'networkidle' });
    await v2.waitForWorkerReady();
    await v2.waitForChartCount(1);

    expect(await v2.getToolbarState()).to.deep.equal({
      dateMode: true,
      evenXAxisSpacing: true,
      showZero: true,
      hoverMode: 'both',
      showDots: false,
      showRegressions: false,
      normalizeCentre: 'first',
    });

    const tourResult = await v2.runInteractiveTour();
    expect(tourResult.visitedTitles).to.deep.equal(['Dynamic Setup', 'Faceted Search Bar']);
    expect(tourResult.activeButtonsCountAfterClose).to.equal(0);
  });

  it('displays anomaly details in trace-chart-tooltip-sk and executes tooltip triage and Pinpoint bisect', async () => {
    await mock.reset('chrome');
    const v2 = new ExploreV2Page(testBed.page, testBed.baseUrl);
    await v2.goto();

    await v2.addQueryFilters(0, {
      benchmark: 'jetstream3.crossbench',
      bot: 'mac-m3-pro-perf',
      test: 'Total',
    });
    await v2.waitForChartCount(1);

    const tooltipInfo = await v2.hoverAnomalyOnChart(0, 67135);
    expect(tooltipInfo.text).to.include('67135');
    expect(tooltipInfo.text).to.include('Regression');
    expect(tooltipInfo.text).to.include('135');
    expect(tooltipInfo.text).to.include('107');
    expect(tooltipInfo.hasTriageMenu).to.be.true;
    expect(tooltipInfo.hasBisectButton).to.be.true;

    await v2.triageTooltipAnomaly(0, 'ignore');
    await v2.hoverAnomalyOnChart(0, 67135);
    await v2.triageTooltipAnomaly(0, 'new-bug');
    await v2.hoverAnomalyOnChart(0, 67135);
    await v2.triageTooltipAnomaly(0, 'existing-bug', '65432');
    await v2.hoverAnomalyOnChart(0, 67135);
    await v2.submitTooltipBisect(0);

    expect((await mock.recorded('/_/triage/edit_anomalies')).length).to.be.greaterThan(0);
    expect((await mock.recorded('/_/triage/file_bug')).length).to.equal(1);
    expect((await mock.recorded('/_/triage/associate_alerts')).length).to.equal(1);
    expect((await mock.recorded('/_/bisect/create')).length).to.equal(1);
  });
});
