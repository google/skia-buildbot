import './index';
import sinon from 'sinon';
import { assert } from 'chai';
import { AnomaliesTableSk } from './anomalies-table-sk';
import {
  doRangesOverlap,
  groupAnomalies,
  isSameBenchmark,
  isSameRevision,
  isSameBot,
  isSameTest,
  AnomalyGroupingConfig,
} from './grouping';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly } from '../json';
import fetchMock from 'fetch-mock';

describe('anomalies-table-sk', () => {
  const newInstance = setUpElementUnderTest<AnomaliesTableSk>('anomalies-table-sk');
  fetchMock.config.overwriteRoutes = false;

  let element: AnomaliesTableSk;
  beforeEach(() => {
    window.perf = {
      dev_mode: false,
      instance_url: '',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
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
      // TODO(b/454590264) For now, sql anomalies are disabled. Change to true in the future.
      fetch_anomalies_from_sql: false,
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
      show_bisect_btn: true,
      app_version: 'test-version',
      enable_v2_ui: false,
      extra_links: null,
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
    // TODO(b/454590264) For now, sql anomalies are disabled. Change to true in the future.
    window.perf.fetch_anomalies_from_sql = false;
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
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies);
      assert.equal(element.anomalyList.length, 1);
    });
  });

  describe('open report', () => {
    it('opens a new window with the correct url', async () => {
      const spy = sinon.spy(window, 'open');
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      await element.populateTable(anomalies);
      await element.checkSelectedAnomalies(anomalies);
      await element.openReport();
      sinon.assert.calledOnce(spy);
      sinon.assert.calledWith(spy, '/u/?anomalyIDs=1', '_blank');
    });
  });

  describe('open anomaly group report page', () => {
    it('navigates to anomaly group report page when sql anoms are disabled', async () => {
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
      await element.populateTable(anomalies);

      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledThrice);
    });

    it('navigates to anomaly group report page when sql anomalies are enabled', async () => {
      window.perf.fetch_anomalies_from_sql = true;

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
      await element.populateTable(anomalies);

      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith(`/u/?anomalyIDs=${encodeURIComponent('1,2')}`, '_blank'));
      assert.isTrue(openSpy.calledWith(`/u/?anomalyIDs=${encodeURIComponent('3,4')}`, '_blank'));
      assert.isTrue(openSpy.calledWith(`/u/?anomalyIDs=${encodeURIComponent('5,6')}`, '_blank'));
      assert.isTrue(openSpy.calledThrice);
    });

    it('opens single anomaly group with anomaly id', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1')];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies);

      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?anomalyIDs=1', '_blank'));
      assert.isTrue(openSpy.calledOnce);
    });

    it('opens multi anomaly group with sid when sql anoms are disabled', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies);

      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledOnce);
    });

    it('opens short multi anomaly group with anomalyIDs using sql anomalies', async () => {
      window.perf.fetch_anomalies_from_sql = true;

      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies);

      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledWith(`/u/?anomalyIDs=${encodeURIComponent('1,2')}`, '_blank'));
      assert.isTrue(openSpy.calledOnce);
    });

    it('opens both single and multi anomaly groups when sql anoms are disabled', async () => {
      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 54321, 100, 200, 'master/bot/suite/test3'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies);

      // Check multi-anomaly group.
      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledTwice);
      assert.isTrue(openSpy.calledWith('/u/?sid=test_sid', '_blank'));
      assert.isTrue(openSpy.calledWith('/u/?anomalyIDs=3', '_blank'));
    });

    it('opens both single and multi anomaly groups using sql anomalies', async () => {
      window.perf.fetch_anomalies_from_sql = true;

      const openSpy = sinon.spy(window, 'open');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });

      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 54321, 100, 200, 'master/bot/suite/test3'),
      ];
      element.anomalyList = anomalies;
      await element.populateTable(anomalies);

      // Check multi-anomaly group.
      element.checkSelectedAnomalies(anomalies);

      await element.openAnomalyGroupReportPage();

      assert.isTrue(openSpy.calledTwice);
      assert.isTrue(openSpy.calledWith(`/u/?anomalyIDs=${encodeURIComponent('1,2')}`, '_blank'));
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

  describe('groupAnomalies function', () => {
    let anomalies: Anomaly[];
    beforeEach(() => {
      anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot1/suite1/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot2/suite1/test2'),
        dummyAnomaly('3', 0, 300, 400, 'master/bot1/suite1/test1'),
        dummyAnomaly('4', 0, 350, 450, 'master/bot1/suite1/test2'),
        dummyAnomaly('5', 0, 500, 600, 'master/bot2/suite2/test3'),
        dummyAnomaly('6', 0, 500, 600, 'master/bot2/suite2/test3'),
        dummyAnomaly('7', 0, 700, 800, 'master/bot1/suite2/test4'),
      ];
    });

    it('groups by bug_id first', () => {
      const config: AnomalyGroupingConfig = {
        revisionMode: 'ANY',
        groupBy: new Set(),
        groupSingles: false,
      };
      const groups = groupAnomalies(anomalies, config);
      const bugGroup = groups.find((g) => g.anomalies.some((a) => a.id === '1' || a.id === '2'));
      assert.isDefined(bugGroup);
      assert.lengthOf(bugGroup!.anomalies, 2);
    });

    it('handles revisionMode: EXACT', () => {
      const config: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(),
        groupSingles: false,
      };
      const groups = groupAnomalies(anomalies, config);
      // anoms 5 and 6 should be in a group.
      const exactGroup = groups.find((g) => g.anomalies.some((a) => a.id === '5' || a.id === '6'));
      assert.isDefined(exactGroup);
      assert.lengthOf(exactGroup!.anomalies, 2);
      // check that 3 and 4 are not grouped.
      const anom3group = groups.find((g) => g.anomalies.some((a) => a.id === '3'));
      assert.lengthOf(anom3group!.anomalies, 1);
    });

    it('handles revisionMode: OVERLAPPING', () => {
      const config: AnomalyGroupingConfig = {
        revisionMode: 'OVERLAPPING',
        groupBy: new Set(),
        groupSingles: false,
      };
      const groups = groupAnomalies(anomalies, config);
      // anoms 3 and 4 should be in a group.
      const overlapGroup = groups.find((g) =>
        g.anomalies.some((a) => a.id === '3' || a.id === '4')
      );
      assert.isDefined(overlapGroup);
      assert.lengthOf(overlapGroup!.anomalies, 2);
    });

    it('handles revisionMode: ANY', () => {
      const config: AnomalyGroupingConfig = {
        revisionMode: 'ANY',
        groupBy: new Set(),
        groupSingles: false,
      };
      const groups = groupAnomalies(anomalies, config);
      // all non-bug anomalies are grouped
      const anyGroup = groups.find((g) => g.anomalies.length === 5); // 7 total - 2 with bug_id
      assert.isDefined(anyGroup);
    });

    it('splits revision groups by BOT', () => {
      const localAnomalies = [
        dummyAnomaly('3', 0, 300, 400, 'master/bot1/suite1/test1'),
        dummyAnomaly('4', 0, 300, 400, 'master/bot2/suite1/test1'),
      ];
      const config: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(['BOT']),
        groupSingles: false,
      };
      const groups = groupAnomalies(localAnomalies, config);
      assert.lengthOf(groups, 2); // Split into two groups of 1.
      assert.lengthOf(groups[0].anomalies, 1);
      assert.lengthOf(groups[1].anomalies, 1);
    });

    it('groups singles by BENCHMARK when groupSingles is true', () => {
      const localAnomalies = [
        dummyAnomaly('1', 0, 100, 100, 'master/bot1/suite1/test1'),
        dummyAnomaly('2', 0, 200, 200, 'master/bot1/suite1/test2'),
      ];
      const config: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(['BENCHMARK']),
        groupSingles: true,
      };
      const groups = groupAnomalies(localAnomalies, config);
      assert.lengthOf(groups, 1);
      assert.lengthOf(groups[0].anomalies, 2);
    });

    it('does not group singles when groupSingles is false', () => {
      const localAnomalies = [
        dummyAnomaly('1', 0, 100, 100, 'master/bot1/suite1/test1'),
        dummyAnomaly('2', 0, 200, 200, 'master/bot1/suite1/test2'),
      ];
      const config: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(['BENCHMARK']),
        groupSingles: false,
      };
      const groups = groupAnomalies(localAnomalies, config);
      assert.lengthOf(groups, 2);
    });

    it('groups by multiple criteria (BOT and BENCHMARK)', () => {
      const localAnomalies = [
        dummyAnomaly('1', 0, 100, 100, 'master/bot1/suite1/test1'), // Group A
        dummyAnomaly('2', 0, 100, 100, 'master/bot1/suite1/test2'), // Group A
        dummyAnomaly('3', 0, 100, 100, 'master/bot2/suite1/test1'), // single
        dummyAnomaly('4', 0, 100, 100, 'master/bot1/suite2/test1'), // single
      ];
      const config: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(['BOT', 'BENCHMARK']),
        groupSingles: false,
      };
      const groups = groupAnomalies(localAnomalies, config);
      assert.lengthOf(groups, 3);
      const groupA = groups.find((g) => g.anomalies.length === 2);
      assert.isDefined(groupA);
    });
  });

  describe('do ranges overlap', () => {
    it('returns true if ranges overlap', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 150, 250, '');
      assert.isTrue(doRangesOverlap(a, b));
    });

    it('returns false if ranges do not overlap', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 300, 400, '');
      assert.isFalse(doRangesOverlap(a, b));
    });
  });

  describe('is same revision', () => {
    it('returns true if revisions are the same', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 100, 200, '');
      assert.isTrue(isSameRevision(a, b));
    });

    it('returns false if revisions are different', () => {
      const a = dummyAnomaly('1', 0, 100, 200, '');
      const b = dummyAnomaly('2', 0, 100, 201, '');
      assert.isFalse(isSameRevision(a, b));
    });
  });

  describe('is same benchmark', () => {
    it('returns true if benchmarks are the same', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot/suite/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot/suite/test2');
      assert.isTrue(isSameBenchmark(a, b));
    });

    it('returns false if benchmarks are different', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot/suite1/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot/suite2/test2');
      assert.isFalse(isSameBenchmark(a, b));
    });
  });

  describe('is same bot', () => {
    it('returns true if bots are the same', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot1/suite/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot1/suite/test2');
      assert.isTrue(isSameBot(a, b));
    });

    it('returns false if bots are different', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot1/suite1/test1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot2/suite2/test2');
      assert.isFalse(isSameBot(a, b));
    });
  });

  describe('is same test', () => {
    it('returns true if the main test part is the same, ignoring subtests', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot1/suite1/test1/sub1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot2/suite2/test1/sub2'); // Different subtest
      assert.isTrue(isSameTest(a, b));
    });

    it('returns false if tests are different', () => {
      const a = dummyAnomaly('1', 0, 0, 0, 'master/bot1/suite1/test1/sub1');
      const b = dummyAnomaly('2', 0, 0, 0, 'master/bot2/suite2/test2/sub2');
      assert.isFalse(isSameTest(a, b));
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

    it('returns the first anomaly if all bug_ids are -2', () => {
      const anomaly1 = dummyAnomaly('1', -2, 0, 0, '');
      const anomaly2 = dummyAnomaly('2', -2, 0, 0, '');
      const group = { anomalies: [anomaly1, anomaly2], expanded: false };
      const result = element.getReportLinkForSummaryRowBugId(group);
      assert.deepEqual(result, anomaly1);
    });

    it('returns the anomaly with a valid bug_id when mixed with bug_id -2', () => {
      const anomalyWithBug = dummyAnomaly('1', 12345, 0, 0, '');
      const anomalyIgnored = dummyAnomaly('2', -2, 0, 0, '');
      const group = { anomalies: [anomalyIgnored, anomalyWithBug], expanded: false };
      const result = element.getReportLinkForSummaryRowBugId(group);
      assert.deepEqual(result, anomalyWithBug);
    });

    it('returns undefined if all bug_ids are 0 or -2, but not all are -2', () => {
      const anomaly1 = dummyAnomaly('1', 0, 0, 0, '');
      const anomaly2 = dummyAnomaly('2', -2, 0, 0, '');
      const group = { anomalies: [anomaly1, anomaly2], expanded: false };
      const result = element.getReportLinkForSummaryRowBugId(group);
      assert.isUndefined(result);
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
    it('checks the checkboxes for the given anomalies correctly using ID matching', async () => {
      // Create two separate objects with the same ID to ensure we are matching by ID
      // and not by object reference, which was the cause of the original bug.
      const originalAnomaly = dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test');
      const anomalyRequest = dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test');

      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable([originalAnomaly]);

      // Pass the *duplicate* object, which should trigger a selection on the *original* object
      await element.checkSelectedAnomalies([anomalyRequest]);
      await fetchMock.flush(true);

      const checkbox = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      assert.isTrue(checkbox.checked);

      // Verify internal state matches the original object, not the requested copy
      const checkedSet = element.getCheckedAnomalies();
      assert.equal(checkedSet.length, 1);
      assert.strictEqual(checkedSet[0], originalAnomaly);
    });
  });

  describe('toggle children checkboxes', () => {
    it('toggles the checkboxes of all children in a group', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);
      await element.populateTable(anomalies);
      const group = element.anomalyGroups[0];
      const suummarycheckboxid = element.getGroupId(group);
      const summarycheckbox = element.querySelector(
        `input[id^="anomaly-row-"][id$="-${suummarycheckboxid}"]`
      ) as HTMLInputElement;
      summarycheckbox.checked = true;
      element.toggleChildrenCheckboxes(group);

      const checkbox1 = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      const checkbox2 = element.querySelector('[id^="anomaly-row-"][id$="-2"]') as HTMLInputElement;
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
      // Mock shortcut update call to prevent console errors in test
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);
      await element.populateTable(anomalies);

      const headerCheckbox = element.querySelector('[id^="header-checkbox-"]') as HTMLInputElement;
      headerCheckbox.checked = true;
      element.toggleAllCheckboxes();
      const checkbox1 = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      const checkbox2 = element.querySelector('[id^="anomaly-row-"][id$="-2"]') as HTMLInputElement;
      assert.isTrue(checkbox1.checked);
      assert.isTrue(checkbox2.checked);
    });
  });

  describe('checkbox visual states (checked/indeterminate)', () => {
    it('sets group checkbox to INDETERMINATE when partially selected', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies);

      // Select only one anomaly (id: 1)
      await element.checkSelectedAnomalies([anomalies[0]]);

      const group = element.anomalyGroups[0];
      const summaryCheckboxId = element.getGroupId(group);
      const groupCheckbox = element.querySelector(
        `input[id^="anomaly-row-"][id$="-${summaryCheckboxId}"]`
      ) as HTMLInputElement;

      // Group should be Indeterminate, NOT Checked
      assert.isTrue(groupCheckbox.indeterminate, 'Group checkbox should be indeterminate');
      assert.isFalse(groupCheckbox.checked, 'Group checkbox should not be fully checked');
    });

    it('sets group checkbox to CHECKED when all children selected', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies);

      // Select ALL anomalies
      await element.checkSelectedAnomalies(anomalies);

      const group = element.anomalyGroups[0];
      const summaryCheckboxId = element.getGroupId(group);
      const groupCheckbox = element.querySelector(
        `input[id^="anomaly-row-"][id$="-${summaryCheckboxId}"]`
      ) as HTMLInputElement;

      // Group should be Checked, NOT Indeterminate
      assert.isFalse(groupCheckbox.indeterminate, 'Group checkbox should not be indeterminate');
      assert.isTrue(groupCheckbox.checked, 'Group checkbox should be fully checked');
    });

    it('sets header checkbox to INDETERMINATE when partially selected', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies);

      // Select only one anomaly
      await element.checkSelectedAnomalies([anomalies[0]]);

      const headerCheckbox = element.querySelector('[id^="header-checkbox-"]') as HTMLInputElement;

      assert.isTrue(headerCheckbox.indeterminate, 'Header checkbox should be indeterminate');
      assert.isFalse(headerCheckbox.checked, 'Header checkbox should not be fully checked');
    });

    it('sets header checkbox to CHECKED when all selected', async () => {
      const anomalies = [
        dummyAnomaly('1', 0, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 0, 100, 200, 'master/bot/suite/test2'),
      ];
      await element.populateTable(anomalies);

      // Select all anomalies
      await element.checkSelectedAnomalies(anomalies);

      const headerCheckbox = element.querySelector('[id^="header-checkbox-"]') as HTMLInputElement;

      assert.isFalse(headerCheckbox.indeterminate, 'Header checkbox should not be indeterminate');
      assert.isTrue(headerCheckbox.checked, 'Header checkbox should be fully checked');
    });
  });

  describe('open multi graph url', () => {
    it('fetches the url if it does not exist in the map', async () => {
      // A stub is better here because we can control the return value.
      const mockTab = {
        document: { write: () => {} },
        location: { href: '' },
      };
      const openStub = sinon.stub(window, 'open').returns(mockTab as unknown as Window);

      const anomaly = dummyAnomaly('123', 0, 0, 0, 'master/bot/suite/test');
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await fetchMock.flush(true);

      window.history.pushState({}, '', '/a/');

      // CORRECT: Call the function with both arguments.
      // The call to window.open() is now conceptually part of the "user action"
      // that the test is simulating.
      const newTab = window.open('', '_blank');
      await element.openMultiGraphUrl(anomaly, newTab);

      // Assert that window.open was called as expected.
      assert.isTrue(openStub.calledOnce);

      // Assert that the tab was navigated to the correct URL.

      const weekInSeconds = 604800; // 7 * 24 * 60 * 60
      const expectedBegin = 100 - weekInSeconds;
      const expectedEnd = 200 + weekInSeconds;
      const expectedUrl = `/m/?begin=${expectedBegin}&end=${expectedEnd}&request_type=0&shortcut=test_shortcut&totalGraphs=1`;
      assert.include(mockTab.location.href, expectedUrl);
    });
  });

  describe('generate summary row', () => {
    it('correctly summarizes a group of anomalies', async () => {
      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1/sub1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test1/sub2'),
        dummyAnomaly('3', 12345, 120, 220, 'master/bot/suite/test2/sub1'),
      ];
      anomalies[0].is_improvement = true;
      anomalies[0].median_before_anomaly = 100;
      anomalies[0].median_after_anomaly = 150; // +50% improvement
      anomalies[1].is_improvement = false;
      anomalies[1].median_before_anomaly = 100;
      anomalies[1].median_after_anomaly = 80; // -20% regression
      anomalies[2].is_improvement = false;
      anomalies[2].median_before_anomaly = 100;
      anomalies[2].median_after_anomaly = 90; // -10% regression

      await element.populateTable(anomalies);

      const summaryRow = element.querySelector('tr[data-bugid="12345"]');
      assert.isNotNull(summaryRow);

      const cells = summaryRow!.querySelectorAll('td');
      assert.equal(cells[5].textContent?.trim(), 'bot');
      assert.equal(cells[6].textContent?.trim(), 'suite');
      assert.equal(cells[7].textContent?.trim(), 'test*');
      assert.equal(cells[8].textContent?.trim(), '-20%');
      assert.include(cells[8].className, 'regression');
    });
  });

  describe('get checked anomalies', () => {
    it('returns the currently checked anomalies', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      await element.populateTable(anomalies);
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
      await element.populateTable(anomalies);
      element.initialCheckAllCheckbox();
      const checkbox1 = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      const checkbox2 = element.querySelector('[id^="anomaly-row-"][id$="-2"]') as HTMLInputElement;
      assert.isTrue(checkbox1.checked);
      assert.isTrue(checkbox2.checked);
    });

    it('initial all checkboxes including group summary rows', async () => {
      const anomalies = [
        dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test1'),
        dummyAnomaly('2', 12345, 150, 250, 'master/bot/suite/test2'),
        dummyAnomaly('3', 0, 300, 400, 'master/bot/suite/test3'),
      ];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies);
      element.initialCheckAllCheckbox();

      // Check individual anomaly checkboxes
      assert.isTrue(
        (element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement).checked
      );
      assert.isTrue(
        (element.querySelector('[id^="anomaly-row-"][id$="-2"]') as HTMLInputElement).checked
      );
      assert.isTrue(
        (element.querySelector('[id^="anomaly-row-"][id$="-3"]') as HTMLInputElement).checked
      );

      // Check group summary checkboxes (assuming grouping logic creates groups)
      assert.isTrue(
        (element.querySelector('[id^="anomaly-row-"][id$="-group-1-2"]') as HTMLInputElement)
          .checked
      );
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
      await element.populateTable(anomalies);

      // Expand the group to make individual anomaly rows visible.
      const group = element.anomalyGroups[0];
      element.expandGroup(group);

      const checkbox1 = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      assert.isNotNull(checkbox1, 'Checkbox for anomaly 1 should exist.');
      assert.isFalse(checkbox1.checked, 'Checkbox should not be checked initially.');

      // Simulate a single click.
      checkbox1.click();

      assert.isTrue(checkbox1.checked, 'Checkbox should be checked after one click.');
      const checkedAnomalies = element.getCheckedAnomalies();
      assert.deepEqual(checkedAnomalies, [anomalies[0]]);
    });
  });

  describe('individual anomaly styling', () => {
    const createAndTestAnomaly = async (
      id: string,
      isImprovement: boolean,
      before: number,
      after: number,
      expectedClass: 'improvement' | 'regression',
      expectedDelta: string
    ) => {
      const anomaly = dummyAnomaly(id, 12345, 100, 200, 'master/bot/suite/test');
      anomaly.is_improvement = isImprovement;
      anomaly.median_before_anomaly = before;
      anomaly.median_after_anomaly = after;

      await element.populateTable([anomaly]);

      const row = element.querySelector(`[id^="anomaly-row-"][id$="-${id}"]`)!.closest('tr');
      assert.isNotNull(row, `Row for anomaly ${id} should exist.`);

      const deltaCell = row!.querySelector('td:last-child');
      assert.isNotNull(deltaCell, `Delta cell for anomaly ${id} should exist.`);

      assert.include(
        deltaCell!.className,
        expectedClass,
        `Cell should have class '${expectedClass}'`
      );
      assert.notInclude(
        deltaCell!.className,
        expectedClass === 'improvement' ? 'regression' : 'improvement',
        `Cell should not have class '${
          expectedClass === 'improvement' ? 'regression' : 'improvement'
        }'`
      );
      assert.equal(deltaCell!.textContent?.trim(), expectedDelta);
    };

    it('correctly styles a regression where lower is better', async () => {
      await createAndTestAnomaly('1', false, 100, 120, 'regression', '+20%');
    });

    it('correctly styles an improvement where lower is better', async () => {
      await createAndTestAnomaly('2', true, 100, 80, 'improvement', '-20%');
    });

    it('correctly styles a regression where greater is better', async () => {
      await createAndTestAnomaly('3', false, 100, 80, 'regression', '-20%');
    });

    it('correctly styles an improvement where greater is better', async () => {
      await createAndTestAnomaly('4', true, 100, 120, 'improvement', '+20%');
    });
  });

  describe('localStorage config', () => {
    const GROUPING_CONFIG_STORAGE_KEY = 'perf-grouping-config';
    it('loads grouping config from localStorage', () => {
      const storedConfig: AnomalyGroupingConfig = {
        revisionMode: 'EXACT',
        groupBy: new Set(['BOT', 'TEST']),
        groupSingles: false,
      };

      // localStorage stores sets as arrays.
      const storableConfig = {
        ...storedConfig,
        groupBy: Array.from(storedConfig.groupBy),
      };

      localStorage.setItem(GROUPING_CONFIG_STORAGE_KEY, JSON.stringify(storableConfig));

      // Create a new element to trigger connectedCallback and load from storage.
      const newElement = newInstance();

      assert.deepEqual(newElement.currentConfig, storedConfig);

      // Clean up localStorage.
      localStorage.removeItem(GROUPING_CONFIG_STORAGE_KEY);
    });
  });

  describe('summary row styling', () => {
    const createAnomaly = (
      id: string,
      isImprovement: boolean,
      before: number,
      after: number
    ): Anomaly => {
      const anomaly = dummyAnomaly(id, 12345, 100, 200, 'master/bot/suite/test');
      anomaly.is_improvement = isImprovement;
      anomaly.median_before_anomaly = before;
      anomaly.median_after_anomaly = after;
      return anomaly;
    };

    it('shows largest improvement when there are no regressions', async () => {
      const improvement1 = createAnomaly('1', true, 100, 150); // +50%
      const improvement2 = createAnomaly('2', true, 100, 120); // +20%

      await element.populateTable([improvement1, improvement2]);

      const summaryRow = element.querySelector('tr[data-bugid="12345"]');
      assert.isNotNull(summaryRow);
      const summaryCell = summaryRow!.querySelector('td:last-child');
      assert.isNotNull(summaryCell);
      assert.include(summaryCell!.className, 'improvement');
      assert.notInclude(summaryCell!.className, 'regression');
      assert.equal(summaryCell!.textContent?.trim(), '+50%');
    });

    it('shows largest regression when there are only regressions', async () => {
      const regression1 = createAnomaly('1', false, 100, 90); // -10%
      const regression2 = createAnomaly('2', false, 100, 80); // -20%

      await element.populateTable([regression1, regression2]);

      const summaryRow = element.querySelector('tr[data-bugid="12345"]');
      assert.isNotNull(summaryRow);
      const summaryCell = summaryRow!.querySelector('td:last-child');
      assert.isNotNull(summaryCell);
      assert.include(summaryCell!.className, 'regression');
      assert.notInclude(summaryCell!.className, 'improvement');
      assert.equal(summaryCell!.textContent?.trim(), '-20%');
    });

    it('shows largest regression in a mixed group', async () => {
      const improvement = createAnomaly('1', true, 100, 150); // +50%
      const regression1 = createAnomaly('2', false, 100, 90); // -10%
      const regression2 = createAnomaly('3', false, 100, 80); // -20%

      await element.populateTable([improvement, regression1, regression2]);

      const summaryRow = element.querySelector('tr[data-bugid="12345"]');
      assert.isNotNull(summaryRow);
      const summaryCell = summaryRow!.querySelector('td:last-child');
      assert.isNotNull(summaryCell);
      assert.include(summaryCell!.className, 'regression');
      assert.notInclude(summaryCell!.className, 'improvement');
      assert.equal(summaryCell!.textContent?.trim(), '-20%');
    });

    it('shows largest positive regression in a mixed group when lower is better', async () => {
      // Simulates "lower is better". A positive delta is a regression.
      const regression1 = createAnomaly('1', false, 100, 120); // +20% regression
      const regression2 = createAnomaly('2', false, 100, 110); // +10% regression
      const improvement = createAnomaly('3', true, 100, 90); // -10% improvement

      await element.populateTable([regression1, regression2, improvement]);

      const summaryRow = element.querySelector('tr[data-bugid="12345"]');
      assert.isNotNull(summaryRow);
      const summaryCell = summaryRow!.querySelector('td:last-child');
      assert.isNotNull(summaryCell);

      // The class should be 'regression' because regressions exist.
      assert.include(summaryCell!.className, 'regression');
      assert.notInclude(summaryCell!.className, 'improvement');

      // The value should be the largest regression by magnitude, which is +20%.
      assert.equal(summaryCell!.textContent?.trim(), '+20%');
    });
  });

  describe('keyboard shortcuts', () => {
    it('triggers triage actions on key press', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies);

      // Select an anomaly
      const checkbox = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      checkbox.click();

      // Mock triage menu methods
      const triageMenu = element.querySelector('triage-menu-sk') as HTMLElement & {
        fileBug: sinon.SinonSpy;
        ignoreAnomaly: sinon.SinonSpy;
        openExistingBugDialog: sinon.SinonSpy;
        setAnomalies: sinon.SinonSpy;
      };
      triageMenu.fileBug = sinon.spy();
      triageMenu.ignoreAnomaly = sinon.spy();
      triageMenu.openExistingBugDialog = sinon.spy();
      triageMenu.setAnomalies = sinon.spy();

      // Trigger 'p'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'p' }));
      assert.isTrue(triageMenu.fileBug.calledOnce);
      assert.isTrue(element.showPopup);

      // Reset popup
      element.showPopup = false;

      // Trigger 'n'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'n' }));
      assert.isTrue(triageMenu.ignoreAnomaly.calledOnce);
      assert.isTrue(element.showPopup);

      // Reset popup
      element.showPopup = false;

      // Trigger 'e'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'e' }));
      assert.isTrue(triageMenu.openExistingBugDialog.calledOnce);
      assert.isTrue(element.showPopup);
    });

    it('triggers graph actions on key press', async () => {
      const anomalies = [dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test')];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies);

      // Select an anomaly
      const checkbox = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      checkbox.click();

      const openReportSpy = sinon.spy(element, 'openReport');
      const openGroupReportSpy = sinon.spy(element, 'openAnomalyGroupReportPage');

      // Trigger 'g'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }));
      assert.isTrue(openReportSpy.calledOnce);

      // Trigger 'G'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'G' }));
      assert.isTrue(openGroupReportSpy.calledOnce);
    });

    it('triggers graph actions on key press with partially selected group', async () => {
      const anomaly1 = dummyAnomaly('1', 12345, 100, 200, 'master/bot/suite/test');
      const anomaly2 = dummyAnomaly('2', 12345, 100, 200, 'master/bot/suite/test');
      const anomalies = [anomaly1, anomaly2];
      fetchMock.post('/_/shortcut/update', { id: 'test_shortcut' });
      await element.populateTable(anomalies);

      // Select only one anomaly in the group
      const checkbox = element.querySelector('[id^="anomaly-row-"][id$="-1"]') as HTMLInputElement;
      checkbox.click();

      const openGroupReportSpy = sinon.spy(element, 'openAnomalyGroupReportPage');
      const openReportForAnomalyIdsSpy = sinon.spy(element, 'openReportForAnomalyIds');

      // Trigger 'G'
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'G' }));
      assert.isTrue(openGroupReportSpy.calledOnce);

      // Verify it tries to open report for BOTH anomalies in the group
      assert.isTrue(openReportForAnomalyIdsSpy.calledOnce);
      const args = openReportForAnomalyIdsSpy.firstCall.args[0];
      assert.equal(args.length, 2);
    });
  });
});
