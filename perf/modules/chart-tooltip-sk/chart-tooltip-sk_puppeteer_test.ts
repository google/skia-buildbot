import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ChartTooltipSkPO } from './chart-tooltip-sk_po';
import { ElementHandle } from 'puppeteer';
import { assert } from 'chai';

describe('chart-tooltip-sk', () => {
  let testBed: TestBed;
  let chartTooltipSkPO: ChartTooltipSkPO;
  let chartTooltipSk: ElementHandle;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.evaluateOnNewDocument(() => {
      (window as any).perf = {
        instance_url: 'https://chrome-perf.corp.goog',
        commit_range_url: 'https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}',
        key_order: ['config'],
        demo: true,
        radius: 7,
        num_shift: 10,
        interesting: 25,
        step_up_only: false,
        display_group_by: false,
        hide_list_of_commits_on_explore: true,
        notifications: 'none',
        fetch_chrome_perf_anomalies: false,
        feedback_url: '',
        chat_url: '',
        help_url_override: '',
        trace_format: 'chrome',
        need_alert_action: false,
        bug_host_url: 'b',
        git_repo_url: 'https://chromium.googlesource.com/chromium/src',
        keys_for_commit_range: [],
        keys_for_useful_links: [],
        skip_commit_detail_display: false,
        image_tag: 'fake-tag',
        remove_default_stat_value: false,
        enable_skia_bridge_aggregation: false,
        show_json_file_display: true,
        always_show_commit_info: false,
        show_triage_link: false,
        show_bisect_btn: true,
      };
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
    chartTooltipSk = (await testBed.page.$('chart-tooltip-sk'))!;
    if (!chartTooltipSk) {
      throw new Error('chart-tooltip-sk not found');
    }

    chartTooltipSkPO = new ChartTooltipSkPO(chartTooltipSk);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('chart-tooltip-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk');
    });
  });

  describe('click events', () => {
    it('resets the tooltip', async () => {
      await testBed.page.click('#reset-tooltip');
      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-reset');
    });

    it('loads data without anomaly', async () => {
      await testBed.page.click('#load-data-without-anomaly');
      const anomalyDetails = await chartTooltipSkPO.anomalyDetails;
      const content = await testBed.page.content();
      assert.include(content, 'Change');
      assert.isTrue(await (await anomalyDetails).isEmpty());
      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-load-without-anomaly');
    });

    it('loads anomaly data', async () => {
      await testBed.page.click('#load-data-with-anomaly');
      const anomalyDetails = await chartTooltipSkPO.anomalyDetails;
      const content = await testBed.page.content();
      assert.include(content, 'Anomaly');
      assert.isNotEmpty(await anomalyDetails);
      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-load-anomaly-details');
    });

    it('loads commit range data', async () => {
      await testBed.page.click('#load-initial-data');
      const commitRangeLink = await chartTooltipSkPO.commitRangeLink;
      assert.isNotEmpty(await commitRangeLink);
      assert.isNotNull(await commitRangeLink.link);
      await takeScreenshot(testBed.page, 'perf', 'chart-tooltip-sk-load-commit-range');
    });
  });
});
