import './index';
import sinon from 'sinon';
import { assert } from 'chai';
import { AnomaliesTableSk } from './anomalies-table-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly, Timerange } from '../json';
import fetchMock from 'fetch-mock';

describe('anomalies-table-sk', () => {
  const newInstance = setUpElementUnderTest<AnomaliesTableSk>('anomalies-table-sk');
  fetchMock.config.overwriteRoutes = false;

  let element: AnomaliesTableSk;
  beforeEach(() => {
    window.perf = {
      instance_url: '',
      commit_range_url: 'http://example.com/range/{begin}/{end}',
      key_order: ['config'],
      demo: true,
      radius: 7,
      num_shift: 10,
      interesting: 25,
      step_up_only: false,
      display_group_by: true,
      hide_list_of_commits_on_explore: false,
      notifications: 'none',
      fetch_chrome_perf_anomalies: false,
      feedback_url: '',
      chat_url: '',
      help_url_override: '',
      trace_format: '',
      need_alert_action: false,
      bug_host_url: 'https://example.bug.url',
      git_repo_url: '',
      keys_for_commit_range: [],
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: 'fake-tag',
      remove_default_stat_value: false,
      enable_skia_bridge_aggregation: false,
      show_json_file_display: false,
      always_show_commit_info: false,
      show_triage_link: true,
    };

    fetchMock.post('begin:/_/anomalies/group_report', {
      sid: 'test_sid',
      timerange_map: {
        '123': {
          begin: 100,
          end: 200,
        },
      },
    });

    element = newInstance();
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  const dummyAnomaly = (
    id: string,
    bugId: number,
    start: number,
    end: number,
    testPath: string
  ): Anomaly => ({
    id: id,
    test_path: testPath,
    bug_id: bugId,
    start_revision: start,
    end_revision: end,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Test>Component',
    bug_labels: ['TestLabel1', 'TestLabel2'],
    bug_cc_emails: [],
    bisect_ids: [],
  });

  describe('populate table', () => {
    it('populates the table with anomalies', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      const timerangeMap: { [key: string]: Timerange } = {
        '1': { begin: 100, end: 200 },
      };
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies, timerangeMap);
      assert.equal(element.anomalyList.length, 1);
    });
  });

  describe('open report', () => {
    it('opens a new window with the correct url', async () => {
      const spy = sinon.spy(window, 'open');
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      await element.populateTable(anomalies, { '1': { begin: 100, end: 200 } });
      await element.checkSelectedAnomalies(anomalies);
      await element.openReport();
      sinon.assert.calledOnce(spy);
      sinon.assert.calledWith(spy, '/u/?anomalyIDs=1', '_blank');
    });
  });

  describe('open anomaly group report page', () => {
    it('navigates to anomaly group report page', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 0, 300, 400, 'master/bot/suite/test3'),
        dummyAnomaly('4', 0, 350, 450, 'master/bot/suite/test4'),
        dummyAnomaly('5', 0, 500, 600, 'master/bot/suite2/test5'),
        dummyAnomaly('6', 0, 700, 800, 'master/bot/suite2/test6'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies, { '1': { begin: 100, end: 200 } });

      for (const group of element.anomalyGroups) {
        const summaryRowCheckboxId = element.getGroupId(group);
        const groupCheckbox = element.querySelector<HTMLInputElement>(
          `input[id="anomaly-row-${summaryRowCheckboxId}"]`
        );
        groupCheckbox!.checked = true;
      }

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledThrice);
    });

    it('opens single anomaly group with anomaly id', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1')];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies, { '1': { begin: 100, end: 200 } });

      const anomalyCheckbox = element.querySelector<HTMLInputElement>(`input[id="anomaly-row-1"]`);
      anomalyCheckbox!.checked = true;

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?anomalyIDs=1', '_blank'));
      assert.isTrue(openSpy.calledOnce);
    });

    it('opens multi anomaly group with sid', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies, { '1': { begin: 100, end: 200 } });

      const group = element.anomalyGroups[0];
      const summaryRowCheckboxId = element.getGroupId(group);
      const groupCheckbox = element.querySelector<HTMLInputElement>(
        `input[id="anomaly-row-${summaryRowCheckboxId}"]`
      );
      groupCheckbox!.checked = true;

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledOnce);
    });

    it('opens both single and multi anomaly groups', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 54321, 100, 200, 'master/bot/suite/test3'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies, { '1': { begin: 100, end: 200 } });

      // Check multi-anomaly group.
      const multiAnomalyGroup = element.anomalyGroups.find((g) => g.anomalies.length > 1)!;
      const summaryRowCheckboxId = element.getGroupId(multiAnomalyGroup);
      const groupCheckbox = element.querySelector<HTMLInputElement>(
        `input[id="anomaly-row-${summaryRowCheckboxId}"]`
      )!;
      groupCheckbox.checked = true;

      // Check single-anomaly group.
      const singleAnomalyCheckbox =
        element.querySelector<HTMLInputElement>(`input[id="anomaly-row-3"]`)!;
      singleAnomalyCheckbox.checked = true;

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledTwice);
      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledWith('/u/?anomalyIDs=3', '_blank'));
    });
  });

  describe('toggle popup', () => {
    it('toggles the showPopup property', () => {
      assert.isFalse(element.showPopup);
      element.togglePopup();
      assert.isTrue(element.showPopup);
      element.togglePopup();
      assert.isFalse(element.showPopup);
    });
  });

  describe('group anomalies', () => {
    it('groups anomalies by bug id, revision overlap, and benchmark', () => {
      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 0, 300, 400, 'master/bot/suite/test3'),
        dummyAnomaly('4', 0, 350, 450, 'master/bot/suite/test4'),
        dummyAnomaly('5', 0, 500, 600, 'master/bot/suite2/test5'),
        dummyAnomaly('6', 0, 700, 800, 'master/bot/suite2/test6'),
      ];
      element.anomalyList = anomalies;
      element.groupAnomalies();
      assert.lengthOf(element.anomalyGroups, 3);
      assert.lengthOf(element.anomalyGroups[0].anomalies, 2);
      assert.lengthOf(element.anomalyGroups[1].anomalies, 2);
      assert.lengthOf(element.anomalyGroups[2].anomalies, 2);
    });
  });

  describe('do ranges overlap', () => {
    it('returns true if ranges overlap', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 150, 250, '');
      assert.isTrue(element.doRangesOverlap(a, b));
    });

    it('returns false if ranges do not overlap', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 300, 400, '');
      assert.isFalse(element.doRangesOverlap(a, b));
    });
  });

  describe('is same revision', () => {
    it('returns true if revisions are the same', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 100, 200, '');
      assert.isTrue(element.isSameRevision(a, b));
    });

    it('returns false if revisions are different', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 100, 201, '');
      assert.isFalse(element.isSameRevision(a, b));
    });
  });

  describe('is same benchmark', () => {
    it('returns true if benchmarks are the same', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot/suite/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot/suite/test2');
      assert.isTrue(element.isSameBenchmark(a, b));
    });

    it('returns false if benchmarks are different', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot/suite1/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot/suite2/test2');
      assert.isFalse(element.isSameBenchmark(a, b));
    });
  });

  describe('find longest sub test path', () => {
    it('returns the longest common sub test path', () => {
      const anomalies = [
        dummyAnomaly('1', 0, 0, 0, 'master/bot/suite/test1/sub1'),
        dummyAnomaly('2', 0, 0, 0, 'master/bot/suite/test1/sub2'),
      ];
      assert.equal(element.findLongestSubTestPath(anomalies), 'test1/sub*');
    });
  });

  describe('get report link for bug id', () => {
    it('returns a link for a valid bug id', () => {
      const link = element.getReportLinkForBugId(12345);
      assert.isDefined(link);
    });

    it('returns an empty template for bug id 0', () => {
      const link = element.getReportLinkForBugId(0);
      assert.equal(link.strings.at(0), '');
    });

    it('returns a message for bug id -1', () => {
      const link = element.getReportLinkForBugId(-1);
      assert.equal(link.strings[0], 'Invalid Alert');
    });

    it('returns a message for bug id -2', () => {
      const link = element.getReportLinkForBugId(-2);
      assert.equal(link.strings[0], 'Ignored Alert');
    });
  });

  describe('get report link for summary row bug id', () => {
    it('returns the first anomaly with a bug id', () => {
      const anomalyWithBug = dummyAnomaly('1', 12345, 0, 0, '');
      const anomalyWithoutBug = dummyAnomaly('2', 0, 0, 0, '');
      const group = { anomalies: [anomalyWithoutBug, anomalyWithBug], expanded: false };
      const anomaly = element.getReportLinkForSummaryRowBugId(group);
      assert.deepEqual(anomaly, anomalyWithBug);
    });

    it('returns undefined if no anomalies have a bug id', () => {
      const group = { anomalies: [dummyAnomaly('1', 0, 0, 0, '')], expanded: false };
      const anomaly = element.getReportLinkForSummaryRowBugId(group);
      assert.isUndefined(anomaly);
    });
  });

  describe('get row class', () => {
    it('returns the correct class for an expanded parent row', () => {
      const group = { anomalies: [dummyAnomaly('1', 0, 0, 0, '')], expanded: true };
      const rowClass = element.getRowClass(0, group);
      assert.equal(rowClass, 'parent-expanded-row');
    });

    it('returns the correct class for an expanded child row', () => {
      const group = { anomalies: [dummyAnomaly('1', 0, 0, 0, '')], expanded: true };
      const rowClass = element.getRowClass(1, group);
      assert.equal(rowClass, 'child-expanded-row');
    });

    it('returns an empty string for a collapsed row', () => {
      const group = { anomalies: [dummyAnomaly('1', 0, 0, 0, '')], expanded: false };
      const rowClass = element.getRowClass(0, group);
      assert.equal(rowClass, '');
    });
  });

  describe('expand group', () => {
    it('toggles the expanded property of a group', () => {
      const group = { anomalies: [], expanded: false };
      element.expandGroup(group);
      assert.isTrue(group.expanded);
      element.expandGroup(group);
      assert.isFalse(group.expanded);
    });
  });

  describe('compute revision range', () => {
    it('returns the correct range string', () => {
      assert.equal(element.computeRevisionRange(100, 200), '100 - 200');
    });

    it('returns a single number if start and end are the same', () => {
      assert.equal(element.computeRevisionRange(100, 100), '100');
    });

    it('returns an empty string if start or end is null', () => {
      assert.equal(element.computeRevisionRange(null, 100), '');
      assert.equal(element.computeRevisionRange(100, null), '');
    });
  });

  describe('check selected anomalies', () => {
    it('checks the checkboxes for the given anomalies', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      const timerangeMap: { [key: string]: Timerange } = {
        '1': { begin: 100, end: 200 },
      };
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies, timerangeMap);
      await element.checkSelectedAnomalies(anomalies);
      await fetchMock.flush(true);
      const checkbox = element.querySelector('#anomaly-row-1') as HTMLInputElement;
      assert.isTrue(checkbox.checked);
    });
  });

  describe('toggle children checkboxes', () => {
    it('toggles the checkboxes of all children in a group', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      const timerangeMap: { [key: string]: Timerange } = {
        '1': { begin: 100, end: 200 },
      };
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);
      await element.populateTable(anomalies, timerangeMap);
      const group = element.anomalyGroups[0];
      const suummarycheckboxid = element.getGroupId(group);
      const summarycheckbox = element.querySelector(
        `input[id="anomaly-row-${suummarycheckboxid}"]`
      ) as HTMLInputElement;
      summarycheckbox.checked = true;
      element.toggleChildrenCheckboxes(group);

      const checkbox1 = element.querySelector('#anomaly-row-1') as HTMLInputElement;
      const checkbox2 = element.querySelector('#anomaly-row-2') as HTMLInputElement;
      assert.isTrue(checkbox1.checked);
      assert.isTrue(checkbox2.checked);
    });
  });

  describe('toggle all checkboxes', () => {
    it('toggles all checkboxes in the table', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      const timerangeMap: { [key: string]: Timerange } = {
        '1': { begin: 100, end: 200 },
      };
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);
      await element.populateTable(anomalies, timerangeMap);

      const headerCheckbox = element.querySelector('#header-checkbox') as HTMLInputElement;
      headerCheckbox.checked = true;
      element.toggleAllCheckboxes();
      const checkbox1 = element.querySelector('#anomaly-row-1') as HTMLInputElement;
      const checkbox2 = element.querySelector('#anomaly-row-2') as HTMLInputElement;
      assert.isTrue(checkbox1.checked);
      assert.isTrue(checkbox2.checked);
    });
  });

  describe('open multi graph url', () => {
    it('opens the url from the map if it exists', async () => {
      const spy = sinon.spy(window, 'open');
      const anomaly = dummyAnomaly('1', 0, 0, 0, '');
      element.multiChartUrlToAnomalyMap.set('1', 'test_url');
      await element.openMultiGraphUrl(anomaly);
      await fetchMock.flush(true);

      assert.isTrue(spy.calledWith('test_url', '_blank'));
    });

    it('fetches the url if it does not exist in the map', async () => {
      const spy = sinon.spy(window, 'open');
      const anomaly = dummyAnomaly('1', 0, 0, 0, 'master/bot/suite/test');
      fetchMock.post('begin:/_/anomalies/group_report', {
        sid: 'test_sid',
        timerange_map: { '1': { begin: 1, end: 2 } },
      });
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);

      window.history.pushState({}, '', '/a/');
      await element.openMultiGraphUrl(anomaly);
      assert.isTrue(spy.calledOnce);
    });
  });

  describe('get checked anomalies', () => {
    it('returns the currently checked anomalies', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      await element.populateTable(anomalies, {});
      element.checkSelectedAnomalies(anomalies);
      const checked = element.getCheckedAnomalies();
      assert.deepEqual(checked, anomalies);
    });
  });

  describe('fetch group report api', () => {
    it('fetches the group report', async () => {
      fetchMock.post('begin:/_/anomalies/group_report', { sid: 'test_sid' });
      await element.fetchGroupReportApi('1,2,3');
      assert.equal(element.getGroupReportResponse!.sid, 'test_sid');
    });
  });

  describe('generate multi graph url', () => {
    it('generates the correct multi graph url', async () => {
      const anomalies = [dummyAnomaly('1', 0, 0, 0, 'master/bot/suite/test')];
      const timerangeMap = { '1': { begin: 1, end: 2 } };
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      const urls = await element.generateMultiGraphUrl(anomalies, timerangeMap);
      assert.isNotEmpty(urls);
    });
  });

  describe('calculate time range', () => {
    it('calculates the correct time range', () => {
      const timerange = { begin: 1000, end: 2000 };
      const newRange = element.calculateTimeRange(timerange);
      const weekInSeconds = 7 * 24 * 60 * 60;
      assert.equal(newRange[0], (1000 - weekInSeconds).toString());
      assert.equal(newRange[1], (2000 + weekInSeconds).toString());
    });
  });

  describe('initial check all checkbox', () => {
    it('checks all checkboxes', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies, {});
      element.initialCheckAllCheckbox();
      const checkbox1 = element.querySelector('#anomaly-row-1') as HTMLInputElement;
      const checkbox2 = element.querySelector('#anomaly-row-2') as HTMLInputElement;
      assert.isTrue(checkbox1.checked);
      assert.isTrue(checkbox2.checked);
    });

    it('initial all checkboxes including group summary rows', async () => {
      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 0, 300, 400, 'master/bot/suite/test3'),
      ];
      const timerangeMap: { [key: string]: Timerange } = {
        '1': { begin: 100, end: 200 },
        '2': { begin: 150, end: 250 },
        '3': { begin: 300, end: 400 },
      };
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies, timerangeMap);
      element.initialCheckAllCheckbox();

      // Check individual anomaly checkboxes
      assert.isTrue((element.querySelector('#anomaly-row-1') as HTMLInputElement).checked);
      assert.isTrue((element.querySelector('#anomaly-row-2') as HTMLInputElement).checked);
      assert.isTrue((element.querySelector('#anomaly-row-3') as HTMLInputElement).checked);

      // Check group summary checkboxes (assuming grouping logic creates groups)
      assert.isTrue((element.querySelector('#anomaly-row-group-1-2') as HTMLInputElement).checked);
    });
  });

  describe('get group id', () => {
    it('returns the correct group id', () => {
      const group = {
        anomalies: [dummyAnomaly('2', 0, 0, 0, ''), dummyAnomaly('1', 0, 0, 0, '')],
        expanded: false,
      };
      const groupId = element.getGroupId(group);
      assert.equal(groupId, 'group-1-2');
    });
  });

  describe('checkbox interaction', () => {
    it('checks a single anomaly in an expanded group on first click', async () => {
      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies, {});

      // Expand the group to make individual anomaly rows visible.
      const group = element.anomalyGroups[0];
      element.expandGroup(group);

      const checkbox1 = element.querySelector('#anomaly-row-1') as HTMLInputElement;
      assert.isNotNull(checkbox1, 'Checkbox for anomaly 1 should exist.');
      assert.isFalse(checkbox1.checked, 'Checkbox should not be checked initially.');

      // Simulate a single click.
      checkbox1.click();

      assert.isTrue(checkbox1.checked, 'Checkbox should be checked after one click.');
      const checkedAnomalies = element.getCheckedAnomalies();
      assert.deepEqual(checkedAnomalies, [anomalies[0]]);
    });
  });
});
