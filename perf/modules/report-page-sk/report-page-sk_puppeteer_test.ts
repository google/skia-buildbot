import { assert, expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ReportPageSkPO } from './report-page-sk_po';
import { poll } from '../common/puppeteer-test-util';

describe('report-page-sk', () => {
  let testBed: TestBed;
  let reportPageSkPO: ReportPageSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setRequestInterception(true);

    testBed.page.on('request', (request) => {
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

  const openTooltip = async (graphIndex: number) => {
    // Verify that the graph for the pre-selected item is displayed.
    const graph = await reportPageSkPO.getGraph(graphIndex);
    const googleChart = graph.googleChart;
    expect(await googleChart.isEmpty()).to.be.false;

    // Verify the anomaly icon exists on the graph.
    const anomalyRect = await googleChart.applyFnToDOMNode((el) => {
      const anomalyIcon = el.shadowRoot!.querySelector('div.anomaly > .anomaly');
      if (!anomalyIcon) return null;
      const rect = anomalyIcon.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    });
    expect(anomalyRect).to.not.be.null;

    // Click the anomaly icon to open the tooltip window.
    // https://screenshot.googleplex.com/5eFnaKGFVdnFWHp
    await testBed.page.mouse.click(
      anomalyRect!.x + anomalyRect!.width / 2,
      anomalyRect!.y + anomalyRect!.height / 2
    );

    // Wait for the tooltip to become visible.
    const containerPO = graph.chartTooltip.container;
    await poll(async () => {
      if (await containerPO.isEmpty()) return false;
      return await containerPO.applyFnToDOMNode(
        (el: any) => (el as HTMLElement).style.display !== 'none'
      );
    }, 'Tooltip was not visible');

    return graph.chartTooltip;
  };

  it('should render the demo page', async () => {
    await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    // Smoke test.
    expect(await testBed.page.$$('report-page-sk')).to.have.length(1);
  });

  describe('anomalies list', () => {
    it('loads anomalies and creates a graph', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const rowCount = await anomaliesTablePO.getRowCount();
      expect(rowCount).to.equal(4);
      const expectedRows: any[] = [
        {
          bugId: 'Bug ID',
          revisions: 'Revisions ',
          bot: 'Bot',
          testSuite: 'Test Suite ',
          test: 'Test',
          delta: 'Delta %',
        },
        {
          bugId: '',
          revisions: '67129 - 67130',
          bot: 'mac-m1_mini_2020-perf',
          testSuite: 'jetstream2',
          test: 'Babylon.First',
          delta: '+23.6228%',
        },
        {
          separator: 'Other groups, related to requested ones (with overlapping commits range)',
        },
        {
          bugId: '',
          revisions: '67129 - 67130',
          bot: '',
          testSuite: '',
          test: '',
          delta: '+35.1741%',
        },
      ];
      const rows = await anomaliesTablePO.rows;
      for (let i = 0; i < (await rows.length); i++) {
        const row = await rows.item(i);
        const expected = expectedRows[i];
        if (expected.separator) {
          expect(await row.innerText).to.contain(expected.separator);
        } else {
          const cells = await row.bySelectorAll('td, th');
          expect(await (await cells.item(3)).innerText).to.equal(expected.bugId);
          expect(await (await cells.item(4)).innerText).to.equal(expected.revisions);
          expect(await (await cells.item(5)).innerText).to.equal(expected.bot);
          expect(await (await cells.item(6)).innerText).to.equal(expected.testSuite);
          expect(await (await cells.item(7)).innerText).to.equal(expected.test);
          expect(await (await cells.item(8)).innerText).to.equal(expected.delta);
        }
      }

      // The selected anomalies should be checked by default because of selected_keys.
      const graphs = await reportPageSkPO.graphs;
      expect(await graphs.length).to.equal(1);
      // Wait for graphs to fully render
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    });

    it('loads commits container', async () => {
      const commitsDiv = await reportPageSkPO.commonCommitsDiv;
      const commitLinks = await reportPageSkPO.commonCommitLinks;
      assert.isNotNull(commitsDiv);
      // The cid response has one commit slice.
      // Must match with the first commitSlice in `perf/modules/common/test-util.ts`
      expect(await commitLinks.length).to.equal(2);
      expect(await (await commitLinks.item(0)).innerText).to.equal('0d7087e');
      const link1: string = (await (await commitLinks.item(0)).getAttribute('href'))!;
      expect(link1).to.equal(
        `https://skia.googlesource.com/skia/+show/0d7087e5b99087f5945f04dbda7b7a7a4b12e344`
      );
      expect(await (await commitLinks.item(1)).innerText).to.equal('2894e71');
      const link2: string = (await (await commitLinks.item(1)).getAttribute('href'))!;
      expect(link2).to.equal(
        `https://skia.googlesource.com/skia/+show/2894e7194406ad8014d3e85b39379ca0e4607ead`
      );
    });
  });

  describe('graph interactions', () => {
    it('synchronizes x-axis toggle across graphs', async () => {
      const graphs = await reportPageSkPO.graphs;
      // there's 1 selected anomaly, therefore one graph will be loaded initially
      // See `GROUP_REPORT_RESPONSE.selected_keys` in `perf/modules/common/test-util.ts`
      expect(await graphs.length).to.equal(1);

      const graph1PO = await reportPageSkPO.getGraph(0);

      // Initial state should be 'commit'
      expect(await graph1PO.getXAxisDomain()).to.equal('commit');
    });

    it('should draw graph for the pre-selected anomaly', async () => {
      const tooltipPO = await openTooltip(0);
      const containerPO = tooltipPO.container;

      await poll(async () => {
        if (await containerPO.isEmpty()) return false;
        const visible = await containerPO.applyFnToDOMNode(
          (el) => (el as HTMLElement).style.display !== 'none'
        );
        if (!visible) return false;
        return (await containerPO.innerText).includes('67130');
      }, 'Tooltip was not visible or did not contain expected data');

      const tooltipText = await containerPO.innerText;

      expect(tooltipText).to.contain('Default [Anomaly]');
      const lines = tooltipText.split('\n').filter((l) => l.trim() !== '');
      const tooltipData: { [key: string]: string } = {};
      lines.forEach((line) => {
        const match = line.match(
          /^(Date|Value|Point Range|Anomaly Range|Anomaly|Median|Previous)\s+(.*)$/
        );
        if (match) {
          tooltipData[match[1]] = match[2].trim();
        }
      });
      expect(tooltipData).to.deep.equal({
        Date: 'Tue, 27 Jun 2023 13:32:43 GMT',
        Value: '75.2',
        'Point Range': '67130',
        'Anomaly Range': '67129 - 67130',
        Anomaly: 'Regression',
        Median: '75.2',
        Previous: '60.8302 [+23.6228%]',
      });
    });

    it('verify params are displayed when collapse button is clicked', async () => {
      // https://screenshot.googleplex.com/HepzKN63UYWfnzf
      const graph = await reportPageSkPO.getGraph(0);
      const collapseButton = graph.collapseButton;
      await collapseButton.click();

      const paramsTab = graph.paramsTab;
      expect(await paramsTab.innerText).to.equal('Params');

      const paramset = graph.paramsetSk;
      expect(await paramset.isEmpty()).to.be.false;

      const expectedParams = [
        { key: 'benchmark', value: 'jetstream2' },
        { key: 'bot', value: 'mac-m1_mini_2020-perf' },
        { key: 'master', value: 'ChromiumPerf' },
        { key: 'subtest_1', value: 'JetStream2' },
        { key: 'test', value: 'Babylon.First' },
      ];

      for (const param of expectedParams) {
        const keyElem = await paramset.bySelector(`th[data-key="${param.key}"]`);
        expect(await keyElem.isEmpty()).to.be.false;
        const valElem = await paramset.bySelector(
          `div[data-key="${param.key}"][data-value="${param.value}"]`
        );
        expect(await valElem.isEmpty()).to.be.false;
      }
    });

    it('removes graph when anomaly is unchecked', async () => {
      // https://screenshot.googleplex.com/7gcAgBMLxcqH8dB
      const graphs = await reportPageSkPO.graphs;
      expect(await graphs.length).to.equal(1);

      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      await anomaliesTablePO.clickCheckbox(1);

      await poll(async () => {
        const currentGraphs = await reportPageSkPO.graphs;
        return (await currentGraphs.length) === 0;
      }, 'Graph should be removed');
    });

    it('select all anomalies to show graphs', async () => {
      // https://screenshot.googleplex.com/68yTUQ45kAiQeHD
      const graphs = await reportPageSkPO.graphs;
      expect(await graphs.length).to.equal(1);

      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      await anomaliesTablePO.clickCheckbox(0);

      await poll(async () => {
        const currentGraphs = await reportPageSkPO.graphs;
        return (await currentGraphs.length) === 2;
      }, 'Two graphs should be displayed');
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

  describe('graph header', () => {
    it('verify Load Test Picker header button in the graph', async () => {
      const graph = await reportPageSkPO.getGraph(0);
      const link = await graph.bySelector('#chartHeader > a');
      expect(await link.isEmpty()).to.be.false;
      const href = await link.getAttribute('href');
      expect(href).to.not.be.null;
      expect(href!).to.contain('/m/?begin=1687265456&end=1687961973');

      const exploreButton = await link.bySelector('md-icon-button');
      expect(await exploreButton.isEmpty()).to.be.false;
      const title = await exploreButton.getAttribute('title');
      expect(title?.toLowerCase()).to.equal('open in multigraph');
    });

    it('verify Show Zero on Axis header button in the graph', async () => {
      const graph = await reportPageSkPO.getGraph(0);
      const showZeroButton = await graph.bySelector(
        '#chartHeader md-icon-button[title="Show Zero on Axis"]'
      );
      expect(await showZeroButton.isEmpty()).to.be.false;
      const icon = await showZeroButton.bySelector('md-icon');
      expect(await icon.innerText).to.equal('hide_source');
    });

    it('verify Add Chart to Favorites header button in the graph', async () => {
      const graph = await reportPageSkPO.getGraph(0);
      const favButton = await graph.bySelector(
        '#chartHeader md-icon-button[title="Add Chart to Favorites"]'
      );
      expect(await favButton.isEmpty()).to.be.false;
      const icon = await favButton.bySelector('md-icon');
      expect(await icon.innerText).to.equal('favorite');
    });

    it('verify Show Settings Dialog header button in the graph', async () => {
      const graph = await reportPageSkPO.getGraph(0);
      const settingsButton = await graph.bySelector(
        '#chartHeader md-icon-button[title="Show Settings Dialog"]'
      );
      expect(await settingsButton.isEmpty()).to.be.false;
      const icon = await settingsButton.bySelector('md-icon');
      expect(await icon.innerText).to.equal('settings');
    });

    it('verify graph title content', async () => {
      // https://screenshot.googleplex.com/6JP5sC9Pnu8cJn8
      const graph = await reportPageSkPO.getGraph(0);
      const graphTitle = await graph.bySelector('#graphTitle');
      expect(await graphTitle.isEmpty()).to.be.false;

      const columns = await graphTitle.bySelectorAll('.column');
      expect(await columns.length).to.equal(4);

      const expectedData = [
        { key: 'benchmark', value: 'jetstream2' },
        { key: 'bot', value: 'mac-m1_mini_2020-perf' },
        { key: 'master', value: 'ChromiumPerf' },
        { key: 'test', value: 'Babylon.First' },
      ];

      for (let i = 0; i < expectedData.length; i++) {
        const column = await columns.item(i);
        const param = await column.bySelector('.param');
        const value = await column.bySelector('.hover-to-show-text');

        expect(await param.innerText).to.equal(expectedData[i].key);
        expect(await value.innerText).to.equal(expectedData[i].value);
      }
    });
  });

  describe('tooltip actions', () => {
    it('verify new bug button', async () => {
      // https://screenshot.googleplex.com/577QkbXf2BVShas
      const tooltipPO = await openTooltip(0);
      const triageMenuSkPO = await tooltipPO.getTriageMenu;
      await triageMenuSkPO.newBugButton.click();
      await testBed.page.$('new-bug-dialog-sk');

      // By mocking '/_/triage/file_bug', expecting '358011161' bug number.
      await poll(async () => {
        const link = await tooltipPO.container.bySelector('a[href="b/358011161"]');
        return !(await link.isEmpty()) && (await link.innerText).includes('358011161');
      }, 'Tooltip should show bug ID 358011161');

      const unassociateBtn = await tooltipPO.container.bySelector('#unassociate-bug-button');
      expect(await unassociateBtn.isEmpty()).to.be.false;

      // TODO(b/483690789): Verify the anomaly color changes from yellow to red.
    });

    it('verify existing bug button', async () => {
      // https://screenshot.googleplex.com/aiREmTYFPgQ54sB
      const tooltipPO = await openTooltip(0);
      const triageMenuSkPO = await tooltipPO.getTriageMenu;
      await triageMenuSkPO.existingBugButton.click();

      const existingBugDialogSk = await testBed.page.$('existing-bug-dialog-sk');
      expect(existingBugDialogSk).to.not.be.null;

      const dialog = await existingBugDialogSk!.$('dialog#existing-bug-dialog');
      expect(dialog).to.not.be.null;

      const form = await dialog!.$('form#existing-bug-form');
      expect(form).to.not.be.null;

      const input = await form!.$('input#bug_id');
      expect(input).to.not.be.null;
      await input!.evaluate((el) => ((el as HTMLInputElement).value = '358011161'));

      const submitBtn = await form!.$('button#file-button');
      expect(await (await submitBtn!.getProperty('innerText')).jsonValue()).to.equal('Submit');

      // TODO(b/483690789): Verify the anomaly color changes from yellow to red.
    });
  });
});
