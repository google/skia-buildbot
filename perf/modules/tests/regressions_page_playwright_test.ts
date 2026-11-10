import { expect } from 'chai';
import { loadPlaywrightTestBed, PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';
import { MockServerClient } from './mock_client';
import { RegressionsPage } from './pages/regressions_page';
import { TriageMenu } from './pages/triage_menu';

describe('Regressions page (/a) workflows', () => {
  let testBed: PlaywrightTestBed;
  let mock: MockServerClient;

  before(async () => {
    testBed = await loadPlaywrightTestBed();
    mock = new MockServerClient(testBed);
  });

  it('filters by V8 JavaScript Perf, toggles Show Triaged and Show Improvements, and opens graphs via trending icons and Graph Selected', async () => {
    await mock.reset('v8');
    await mock.pinExploreV2(false);
    const regressions = new RegressionsPage(testBed.page, testBed.baseUrl);
    await regressions.goto();

    await regressions.selectSheriff('V8 JavaScript Perf');
    expect(await testBed.page.title()).to.equal('Regressions (2 untriaged)');
    expect(await regressions.rows.count()).to.equal(2);
    await regressions.anomaliesTable
      .getByText('jetstream3.crossbench')
      .first()
      .waitFor({ state: 'visible' });
    await regressions.anomaliesTable
      .getByText('speedometer3.crossbench')
      .first()
      .waitFor({ state: 'visible' });

    await regressions.setShowTriaged(true);
    expect(await testBed.page.title()).to.equal('Regressions (3 total)');
    expect(await regressions.rows.count()).to.equal(3);
    const triagedBugLink = regressions.anomaliesTable.locator('a[href="http://b/12345"]');
    await triagedBugLink.waitFor({ state: 'visible' });
    expect((await triagedBugLink.textContent())?.trim()).to.equal('12345');

    await regressions.setShowImprovements(true);
    expect(await testBed.page.title()).to.equal('Regressions (5 total)');
    await regressions.anomaliesTable
      .locator('tr.summary-row td.improvement')
      .first()
      .waitFor({ state: 'visible' });

    const summaryUrls = await regressions.captureOpenedUrls(() =>
      regressions.clickSummaryRowTrendingIcon()
    );
    expect(summaryUrls.some((u) => u.includes('/u/?sid=mock-shortcut-'))).to.be.true;

    await regressions.setShowImprovements(false);
    await regressions.setShowTriaged(false);
    expect(await testBed.page.title()).to.equal('Regressions (2 untriaged)');

    const singleUrls = await regressions.captureOpenedUrls(() =>
      regressions.clickSingleRowTrendingIcon('123')
    );
    expect(singleUrls.length).to.equal(1);
    expect(singleUrls[0]).to.include('/m/?begin=');
    expect(singleUrls[0]).to.include('shortcut=');

    const selectedUrls = await regressions.captureOpenedUrls(async () => {
      await regressions.checkAnomaly('123');
      await regressions.checkAnomaly('789');
      await regressions.clickGraphSelected();
    });
    expect(selectedUrls.some((u) => u.includes('/u/?sid=mock-shortcut-'))).to.be.true;

    const regrShortcuts = await mock.recorded<{ anomalyIDs: string }>(
      '/_/anomalies/calculate_regr_shortcut'
    );
    expect(regrShortcuts.length).to.be.greaterThan(0);
    expect(regrShortcuts[regrShortcuts.length - 1].anomalyIDs).to.include('123');
    expect(regrShortcuts[regrShortcuts.length - 1].anomalyIDs).to.include('789');

    const groupUrls = await regressions.captureOpenedUrls(() =>
      regressions.clickGraphSelectedByGroup()
    );
    expect(groupUrls).to.include('/u/?anomalyIDs=123');
    expect(groupUrls).to.include('/u/?anomalyIDs=789');
  });

  it('routes single-row trending icon links to /m in v1 mode and switches to /e2 via OPT-IN', async () => {
    await mock.reset('v8');
    await mock.pinExploreV2(false);
    const regressions = new RegressionsPage(testBed.page, testBed.baseUrl);
    await regressions.goto();
    await regressions.selectSheriff('V8 JavaScript Perf');

    const [singleGraphUrl] = await regressions.captureOpenedUrls(() =>
      regressions.clickSingleRowTrendingIcon('123')
    );
    expect(
      await testBed.page
        .locator('perf-scaffold-sk a', { hasText: 'MultiGraph' })
        .first()
        .getAttribute('href')
    ).to.equal('/m');

    const targetGraphUrl = singleGraphUrl.startsWith('http')
      ? singleGraphUrl
      : `${testBed.baseUrl}${singleGraphUrl}`;
    await testBed.page.goto(targetGraphUrl, { waitUntil: 'networkidle' });
    expect(testBed.page.url()).to.include('/m/?begin=');
    expect(testBed.page.url()).to.include('shortcut=');

    await testBed.page.getByRole('button', { name: 'OPT-IN (Switch to V2)' }).click();
    await testBed.page.waitForURL((u) => u.pathname.startsWith('/e2'), { timeout: 10000 });
    expect(testBed.page.url()).to.include('/e2');
    expect(
      await testBed.page
        .locator('perf-scaffold-sk a', { hasText: 'MultiGraph' })
        .first()
        .getAttribute('href')
    ).to.equal('/e2');
  });

  it('triages anomalies via Ignore, New Bug, and Existing Bug with immediate in-place Bug ID updates and reload persistence', async () => {
    await mock.reset('chrome');
    const regressions = new RegressionsPage(testBed.page, testBed.baseUrl);
    await regressions.goto();
    await regressions.installWindowOpenSpy();
    await regressions.selectSheriff('Chromium Perf Sheriff');

    expect(await regressions.rows.count()).to.equal(3);

    const triageMenu = new TriageMenu(testBed.page, 'regressions-page-sk anomalies-table-sk');

    expect(await regressions.triageAnomaly(triageMenu, '123', 'ignore')).to.include(
      'Ignored Alert'
    );
    const ignoreMutations = await mock.recorded<{ action: string; keys: Array<string | number> }>(
      '/_/triage/edit_anomalies'
    );
    expect(ignoreMutations).to.have.lengthOf(1);
    expect(ignoreMutations[0].action).to.equal('IGNORE');
    expect(ignoreMutations[0].keys.map(String)).to.include('123');

    expect(await regressions.triageAnomaly(triageMenu, '456', 'new-bug')).to.include('12345');
    const fileBugMutations = await mock.recorded<{ title: string; keys: Array<string | number> }>(
      '/_/triage/file_bug'
    );
    expect(fileBugMutations).to.have.lengthOf(1);
    expect(fileBugMutations[0].keys.map(String)).to.include('456');
    expect(fileBugMutations[0].title).to.include('jetstream3.crossbench');

    expect(await regressions.triageAnomaly(triageMenu, '789', 'existing-bug', 54321)).to.include(
      '54321'
    );
    const associateMutations = await mock.recorded<{
      bug_id: number;
      keys: Array<string | number>;
    }>('/_/triage/associate_alerts');
    expect(associateMutations).to.have.lengthOf(1);
    expect(associateMutations[0].bug_id).to.equal(54321);
    expect(associateMutations[0].keys.map(String)).to.include('789');

    await testBed.page.reload({ waitUntil: 'networkidle' });
    expect(await regressions.rows.count()).to.equal(0);

    await regressions.setShowTriaged(true);
    expect(await regressions.rows.count()).to.equal(3);
    await regressions.anomaliesTable.getByText('Ignored Alert').waitFor({ state: 'visible' });
    await regressions.anomaliesTable
      .locator('a[href="http://b/12345"]')
      .waitFor({ state: 'visible' });
    await regressions.anomaliesTable
      .locator('a[href="http://b/54321"]')
      .waitFor({ state: 'visible' });
  });
});
