import { assert, expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ReportPageSkPO } from './report-page-sk_po';
import { anomalies, defaultConfig } from './test_data';
import { Page } from 'puppeteer';

describe('report-page-sk', () => {
  let testBed: TestBed;
  let reportPageSkPO: ReportPageSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setRequestInterception(true);

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
        fetch_anomalies_from_sql: false,
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
        show_json_file_display: false,
        always_show_commit_info: false,
        show_triage_link: false,
        show_bisect_btn: true,
      };
    });

    const apiMocks = [
      {
        name: 'CID Details',
        matches: (url: string) => url.endsWith('/_/cid/'),
        response: {
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            commitSlice: [
              {
                offset: 64809,
                hash: '3b8de1058a896b613b451db1b6e2b28d58f64a4a',
                ts: 1676307170,
                author: 'Joe Gregorio <jcgregorio@google.com>',
                message: 'Add -prune to gazelle_update_repo run of gazelle.',
                url:
                  'https://skia.googlesource.com/skia/' +
                  '+show/3b8de1058a896b613b451db1b6e2b28d58f64a4a',
              },
            ],
            logEntry:
              'commit 3b8de1058a896b613b451db1b6e2b28d58f64a4a\nAuthor: Joe Gregorio' +
              '<jcgregorio@google.com>\nDate:    Mon Feb 13 10:20:19 2023 -0500\n\n',
          }),
        },
      },
      {
        name: 'Anomalies Group Report',
        matches: (url: string, method: string) =>
          url.endsWith('/_/anomalies/group_report') && method === 'POST',
        response: {
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            anomaly_list: anomalies,
            timerange_map: {
              '1': { begin: 1676307170, end: 1676307170 },
              '2': { begin: 1676307170, end: 1676307170 },
            },
            selected_keys: ['1', '2'],
            is_commit_number_based: true,
          }),
        },
      },
      {
        name: 'Get Defaults',
        matches: (url: string, method: string) => url.endsWith('/_/defaults/') && method === 'GET',
        response: {
          status: 200,
          contentType: 'application/json',
          // Provide a realistic default QueryConfig structure. Adjust as needed.
          body: JSON.stringify({
            defaultConfig,
          }),
        },
      },
      {
        name: 'Login Status',
        matches: (url: string) => url.endsWith('/_/login/status'),
        response: {
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ email: 'test@google.com', Roles: ['editor'] }),
        },
      },
      {
        name: 'Bisect Create',
        matches: (url: string) => url.endsWith('/_/bisect/create'),
        response: {
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ jobId: '123456', jobUrl: 'http://example.com' }),
        },
      },
    ];

    testBed.page.on('request', (request) => {
      const url = request.url();
      const method = request.method();

      for (const mock of apiMocks) {
        if (mock.matches(url, method)) {
          request.respond(mock.response);
          return;
        }
      }

      request.continue();
    });

    await testBed.page.goto(testBed.baseUrl);
    const reportPageSk = await testBed.page.$('report-page-sk');
    if (!reportPageSk) {
      throw new Error('report-page-sk not found');
    }
    reportPageSkPO = new ReportPageSkPO(reportPageSk);
    await testBed.page.setViewport({ width: 1200, height: 800 });
    await testBed.page.waitForSelector('#loading-spinner', { hidden: true });
  });

  afterEach(async () => {
    testBed.page.removeAllListeners('request');
    await testBed.page.setRequestInterception(false);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('report-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    });

    it('loads anomalies and creates a graph', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const rowCount = await anomaliesTablePO.getRowCount();
      //TODO(jiaxindong) b/460335412 investigate if generate duplicate rows
      expect(rowCount).to.equal(6);

      // The selected anomalies should be checked by default because of selected_keys.
      const graphs = await reportPageSkPO.graphs;
      expect(await graphs.length).to.equal(2);
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    });

    it('loads commits container', async () => {
      const commitsDiv = await reportPageSkPO.commonCommitsDiv;
      const commitLinks = await reportPageSkPO.commonCommitLinks;
      assert.isNotNull(commitsDiv);
      // the cid response has one commit slice.
      expect(await commitLinks.length).to.equal(1);
      const link: string = (await (await commitLinks.item(0)).getAttribute('href'))!;
      expect(link).to.equal(
        `https://skia.googlesource.com/skia/+show/3b8de1058a896b613b451db1b6e2b28d58f64a4a`
      );
    });
  });

  describe('graph interactions', () => {
    it('synchronizes x-axis toggle across graphs', async () => {
      const graphs = await reportPageSkPO.graphs;
      // there're 2 selected anomalies, therefore two graphs will be loaded initially
      expect(await graphs.length).to.equal(2);

      const graph1PO = await reportPageSkPO.getGraph(0);
      const graph2PO = await reportPageSkPO.getGraph(1);

      // Initial state should be 'commit'
      expect(await graph1PO.getXAxisDomain()).to.equal('commit');
      expect(await graph2PO.getXAxisDomain()).to.equal('commit');
    });
  });

  describe('anomalies table interactions', () => {
    it('should open the trending link in a new tab when the trending icon is clicked', async () => {
      await testBed.page.click('#open-trending-icon');
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      await anomaliesTablePO.clickTrendingIconButton(0);

      const reportPageUrl = await navigateTo(
        testBed.page,
        testBed.baseUrl,
        `/m/?begin=1729042589&end=11739042589&request_type=0&shortcut=1&totalGraphs=1`
      );
      assert.exists(reportPageUrl);
    });

    it('should be able to click expand buttons in the anomalies table', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const expandBtnLength = (await anomaliesTablePO.expandButton).length;

      for (let i = 0; i < (await expandBtnLength); i++) {
        await anomaliesTablePO.clickExpandButton(i);
      }
      // Only 1 Summary row
      const parentExpandRowCount = await anomaliesTablePO.getParentExpandRowCount();
      expect(parentExpandRowCount).to.equal(1);
      // there're 2 child rows total
      const childExpandRowCount = await anomaliesTablePO.getChildRowCount();
      expect(childExpandRowCount).to.equal(2);
      // After clicking expand button, the hidden child rows are visible
      expect(await anomaliesTablePO.isRowHidden(1)).equal(false);
      expect(await anomaliesTablePO.isRowHidden(2)).equal(false);
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk-anomalies-table-checkboxes');
    });

    it('expand button inner text should be equal to the grouped row count', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const rowCount = await anomaliesTablePO.getGroupedRowCount(0);
      const expandBtn = await anomaliesTablePO.expandButton;
      const expandBtnInnerText = (await expandBtn.item(0)).innerText;
      expect(await expandBtnInnerText).to.equal('2\n|\n0');
      expect(rowCount).to.equal(2);
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk-anomalies-table-header-checkbox');
    });

    it('should be able to click header checkbox in the anomalies table', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      await anomaliesTablePO.clickHeaderCheckbox();
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk-anomalies-table-header-checkbox');
    });
  });

  it('should be able to scroll up and down', async () => {
    // Scroll down by 1000px.
    await testBed.page.evaluate(() => window.scrollBy(0, 1000));
    const scrollYAfterScrollDown = await testBed.page.evaluate(() => window.scrollY);
    expect(scrollYAfterScrollDown).to.be.greaterThan(0);

    // Scroll up by 1000px.
    await testBed.page.evaluate(() => window.scrollBy(0, -1000));
    const scrollYAfterScrollUp = await testBed.page.evaluate(() => window.scrollY);
    expect(scrollYAfterScrollUp).to.equal(0);
  });

  async function navigateTo(page: Page, base: string, queryParams = ''): Promise<ReportPageSkPO> {
    await page.goto(`${base}${queryParams}`);
    return new ReportPageSkPO(page.$('report-page-sk'));
  }
});
