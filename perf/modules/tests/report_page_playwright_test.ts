import { expect } from 'chai';
import { loadPlaywrightTestBed, PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';
import { MockServerClient } from './mock_client';
import { BisectDialog } from './pages/bisect_dialog';
import { ReportPage } from './pages/report_page';

describe('Report page (/u) workflows', () => {
  let testBed: PlaywrightTestBed;
  let mock: MockServerClient;

  before(async () => {
    testBed = await loadPlaywrightTestBed();
    mock = new MockServerClient(testBed);
  });

  it('renders requested and overlapping anomaly groups and shows Bisect button on Chrome persona', async () => {
    await mock.reset('chrome');
    await mock.pinExploreV2(false);

    const report = new ReportPage(testBed.page, testBed.baseUrl);
    await report.goto('anomalyIDs=456');

    expect(await testBed.page.title()).to.equal('Report for anomalies: 456');
    await report.overlappingSeparator.waitFor({ state: 'visible', timeout: 10000 });
    expect((await report.overlappingSeparator.textContent())?.trim()).to.equal(
      'Other groups, related to requested ones (with overlapping commits range)'
    );
    await report.commonCommitMessages.first().waitFor({ state: 'visible', timeout: 10000 });
    expect((await report.commonCommitMessages.first().textContent())?.trim()).to.include(
      'Mock commit'
    );

    await report.anomaliesTable
      .getByText('jetstream3.crossbench')
      .first()
      .waitFor({ state: 'visible' });
    await report.anomaliesTable
      .getByText('speedometer3.crossbench')
      .first()
      .waitFor({ state: 'visible' });

    await report.openAnomalyTooltip(67132);
    expect(await report.getTooltipButtonVisibility()).to.deep.equal({
      bisectVisible: true,
      tryJobVisible: true,
    });
  });

  it('hides Bisect and Request Trace buttons in chart-tooltip-sk on V8 persona', async () => {
    await mock.reset('v8');
    await mock.pinExploreV2(false);

    const report = new ReportPage(testBed.page, testBed.baseUrl);
    await report.goto('anomalyIDs=456');
    await report.openAnomalyTooltip(67132);

    expect(await report.getTooltipButtonVisibility()).to.deep.equal({
      bisectVisible: false,
      tryJobVisible: false,
    });
  });

  it('creates a pinpoint bisect job from chart-tooltip-sk on /u and records the payload', async () => {
    await mock.reset('chrome');
    await mock.pinExploreV2(false);

    const report = new ReportPage(testBed.page, testBed.baseUrl);
    await report.goto('anomalyIDs=456');
    await report.openAnomalyTooltip(67132);

    const bisectDialog = new BisectDialog(testBed.page);
    await bisectDialog.openAndSubmit({
      testPath: 'ChromiumPerf/linux-r350-perf/jetstream3.crossbench/Total/JetStream2/Total',
      startCommit: '67131',
      endCommit: '67132',
      bugId: '12345',
      story: 'Total',
      anomalyId: '456',
    });

    const jobLink = bisectDialog.dialog.locator('#pinpoint-job-url');
    await jobLink.waitFor({ state: 'visible', timeout: 10000 });
    expect(await jobLink.getAttribute('href')).to.equal(
      'https://pinpoint-dot-chromeperf.appspot.com/job/pinpoint-mock-job-001'
    );

    const bisectMutations = await mock.recorded<{
      start_git_hash: string;
      end_git_hash: string;
      benchmark: string;
      configuration: string;
      story: string;
      chart: string;
    }>('/_/bisect/create');
    expect(bisectMutations).to.have.lengthOf(1);
    expect(bisectMutations[0].start_git_hash).to.equal('67131');
    expect(bisectMutations[0].end_git_hash).to.equal('67132');
    expect(bisectMutations[0].benchmark).to.equal('jetstream3.crossbench');
    expect(bisectMutations[0].configuration).to.equal('linux-r350-perf');
  });
});
