import './index';
import { assert } from 'chai';
import { ExistingBugDialogSk } from './existing-bug-dialog-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly } from '../json';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

describe('existing-bug-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<ExistingBugDialogSk>('existing-bug-dialog-sk');

  fetchMock.config.overwriteRoutes = false;
  let element: ExistingBugDialogSk;
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

    element = newInstance();
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  const dummyAnomaly = (bugId: number): Anomaly => ({
    id: '1',
    test_path: '',
    bug_id: bugId,
    start_revision: 1234,
    end_revision: 1239,
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
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  });

  describe('open and close dialog', () => {
    it('opens and closes the dialog', () => {
      assert.isFalse(element.isActive);
      element.open();
      assert.isTrue(element.isActive);
    });
  });

  describe('set anomalies', () => {
    it('sets the anomalies and trace names', () => {
      const anomalies = [dummyAnomaly(12345)];
      const traceNames = ['trace1', 'trace2'];
      element.setAnomalies(anomalies, traceNames);
      assert.deepEqual(element._anomalies, anomalies);
      assert.deepEqual(element._traceNames, traceNames);
    });
  });

  describe('add anomaly with existing bug', () => {
    it('successfully adds anomaly to existing bug', async () => {
      const bugId = 12345;
      const anomalies = [dummyAnomaly(bugId)];
      element.setAnomalies(anomalies, []);
      element.querySelector<HTMLInputElement>('#bug_id')!.value = bugId.toString();

      fetchMock.post('/_/triage/associate_alerts', {
        status: 200,
        body: JSON.stringify({}),
      });
      element.addAnomalyWithExistingBug();
      await fetchMock.flush(true);
      // Assert that the dialog closed on success.
      assert.isFalse(element.isActive);
    });

    it('handles error when adding anomaly to existing bug', async () => {
      const bugId = 12345;
      const anomalies = [dummyAnomaly(bugId)];
      element.setAnomalies(anomalies, []);
      element.querySelector<HTMLInputElement>('#bug_id')!.value = bugId.toString();

      fetchMock.post('/_/triage/associate_alerts', 500);

      const event = eventPromise('error-sk');
      element.addAnomalyWithExistingBug();
      await fetchMock.flush(true);
      sinon.stub(window, 'confirm').returns(true);
      const errEvent = await event;
      assert.isDefined(errEvent);
      const errMessage = (errEvent as CustomEvent).detail.message as string;
      assert.strictEqual(
        errMessage,
        'Associate alerts request failed due to an internal server error. Please try again.'
      );
    });

    it('uses bug id as anomaly key if anomalies list is empty', async () => {
      const bugId = 12345;
      // Empty anomalies list
      element.setAnomalies([], []);
      element.querySelector<HTMLInputElement>('#bug_id')!.value = bugId.toString();

      fetchMock.post('/_/triage/associate_alerts', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        // Verify keys contains bugId
        assert.deepEqual(body.keys, [bugId.toString()]);
        return { status: 200, body: JSON.stringify({}) };
      });

      element.addAnomalyWithExistingBug();
      await fetchMock.flush(true);
      assert.isFalse(element.isActive);
    });
  });

  describe('fetch associated bugs', () => {
    it('successfully fetches associated bugs', async () => {
      const anomalies = [dummyAnomaly(12345), dummyAnomaly(67890)];
      element.setAnomalies(anomalies, []);

      fetchMock.post('/_/anomalies/group_report', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          anomalyIDs: anomalies.map((a) => a.id).join(','),
        });
        return { status: 200, body: JSON.stringify({ anomaly_list: anomalies }) };
      });

      fetchMock.post('/_/triage/list_issues', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          IssueIds: [12345, 67890],
        });
        return { status: 200, body: JSON.stringify({ issues: [] }) };
      });

      await element.fetch_associated_bugs();
      await fetchMock.flush(true);

      assert.deepEqual(element._associatedBugIds, new Set([12345, 67890]));
    });

    it('successfully fetches associated bugs with sid', async () => {
      const anomalies = [dummyAnomaly(12345)];
      element.setAnomalies(anomalies, []);

      fetchMock.post('/_/anomalies/group_report', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        if (body.StateId) {
          assert.equal(body.StateId, 'sid');
          return { status: 200, body: JSON.stringify({ anomaly_list: anomalies }) };
        } else {
          assert.deepEqual(body, {
            anomalyIDs: anomalies.map((a) => a.id).join(','),
          });
          return { status: 200, body: JSON.stringify({ sid: 'sid' }) };
        }
      });

      fetchMock.post('/_/triage/list_issues', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          IssueIds: [12345],
        });
        return { status: 200, body: JSON.stringify({ issues: [] }) };
      });

      await element.fetch_associated_bugs();
      await fetchMock.flush(true);

      assert.deepEqual(element._associatedBugIds, new Set([12345]));
    });
  });

  describe('fetch bug titles', () => {
    it('successfully fetches bug titles', async () => {
      const bugId = 12345;
      element._associatedBugIds = new Set([bugId]);

      fetchMock.post('/_/triage/list_issues', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          IssueIds: [bugId],
        });
        return {
          status: 200,
          body: JSON.stringify({
            issues: [{ issueId: bugId.toString(), issueState: { title: 'Test Bug' } }],
          }),
        };
      });

      await element.fetch_bug_titles();
      await fetchMock.flush(true);

      assert.deepEqual(element.bugIdTitleMap, { [bugId]: 'Test Bug' });
    });
  });

  describe('get associated bug list', () => {
    it('correctly extracts bug ids from anomalies', () => {
      const anomalies = [dummyAnomaly(12345), dummyAnomaly(67890), dummyAnomaly(0)];
      element.getAssociatedBugList(anomalies);
      assert.deepEqual(element._associatedBugIds, new Set([12345, 67890]));
    });
  });

  describe('project id toggle', () => {
    it('changes the project id', () => {
      const select = element.querySelector<HTMLSelectElement>(
        '#existing-bug-dialog-select-project'
      )!;
      select.value = 'chromium';
      select.dispatchEvent(new Event('input'));
      assert.equal(element._projectId, 'chromium');
    });
  });

  describe('close dialog', () => {
    it('closes the dialog', () => {
      element.open();
      assert.isTrue(element.isActive);
      element.closeDialog();
      assert.isFalse(element.isActive);
    });
  });
});
