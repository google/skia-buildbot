import './index';
import { assert } from 'chai';
import { TriageMenuSk, NudgeEntry } from './triage-menu-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly } from '../json';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';

describe('triage-menu-sk', () => {
  const newInstance = setUpElementUnderTest<TriageMenuSk>('triage-menu-sk');
  fetchMock.config.overwriteRoutes = false;
  let element: TriageMenuSk;
  beforeEach(async () => {
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
    await element.updateComplete;
  });

  afterEach(() => {
    // Check all mock fetches called at least once and reset.
    assert.isTrue(fetchMock.done());
    fetchMock.restore();
  });

  const dummyAnomaly = (bugId: number): Anomaly => ({
    id: '1',
    test_path: 'test/path/suite/subtest',
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
    bug_component: 'Test>Component',
    bug_labels: ['TestLabel1', 'TestLabel2'],
    bug_cc_emails: [],
    bisect_ids: [],
  });

  describe('set anomalies', () => {
    it('sets the anomalies, trace names, and nudge list', () => {
      const anomalies = [dummyAnomaly(12345)];
      const traceNames = ['trace1', 'trace2'];
      const nudgeList: NudgeEntry[] = [];
      element.setAnomalies(anomalies, traceNames, nudgeList);
      assert.deepEqual(element.anomalies, anomalies);
      assert.deepEqual(element.traceNames, traceNames);
      assert.deepEqual(element.nudgeList, nudgeList);
    });
  });

  describe('file bug', () => {
    it('calls fileNewBug on the new bug dialog', () => {
      const spy = sinon.spy(element.newBugDialog!, 'fileNewBug');
      element.fileBug();
      assert.isTrue(spy.calledOnce);
    });
  });

  describe('open new bug dialog', () => {
    it('calls open on the new bug dialog', () => {
      const spy = sinon.spy(element.newBugDialog!, 'open');
      element.openNewBugDialog();
      assert.isTrue(spy.calledOnce);
    });
  });

  describe('open existing bug dialog', () => {
    it('calls open on the existing bug dialog', () => {
      const spy = sinon.spy(element.existingBugDialog!, 'open');
      element.openExistingBugDialog();
      assert.isTrue(spy.calledOnce);
    });
  });

  describe('ignore anomaly', () => {
    it('calls makeEditAnomalyRequest with IGNORE action', () => {
      const spy = sinon.spy(element, 'makeEditAnomalyRequest');
      element.ignoreAnomaly();
      assert.isTrue(spy.calledWith(element.anomalies, element.traceNames, 'IGNORE'));
    });
  });

  describe('disable nudge', () => {
    it('sets allowNudge to false', () => {
      element.disableNudge();
      assert.isFalse(element.allowNudge);
    });
  });

  describe('toggle buttons', () => {
    it('disables and enables the buttons', async () => {
      element.toggleButtons(false);
      await element.updateComplete;
      const newBugButton = element.querySelector('#new-bug')! as HTMLButtonElement;
      assert.isTrue(newBugButton.disabled);

      element.toggleButtons(true);
      await element.updateComplete;
      assert.isFalse(newBugButton.disabled);
    });
  });

  describe('generate nudge buttons', () => {
    it('generates nudge buttons if nudge is allowed and nudge list is not null', () => {
      const nudgeList: NudgeEntry[] = [new NudgeEntry()];
      element.setAnomalies([], [], nudgeList);
      const buttons = element.generateNudgeButtons();
      assert.isDefined(buttons);
    });

    it('does not generate nudge buttons if nudge is disabled', () => {
      const nudgeList: NudgeEntry[] = [new NudgeEntry()];
      element.setAnomalies([], [], nudgeList);
      element.disableNudge();
      const buttons = element.generateNudgeButtons();
      assert.strictEqual(buttons.strings.at(0), '');
    });

    it('does not generate nudge buttons if nudge list is null', () => {
      element.setAnomalies([], [], null);
      const buttons = element.generateNudgeButtons();
      assert.strictEqual(buttons.strings.at(0), '');
    });
  });

  describe('nudge anomaly', () => {
    it('calls makeNudgeRequest', () => {
      const spy = sinon.spy(element, 'makeNudgeRequest');
      const entry = new NudgeEntry();
      element.nudgeAnomaly(entry);
      assert.isTrue(spy.calledWith(element.anomalies, element.traceNames, entry));
    });
  });

  describe('make edit anomaly request', () => {
    it('sends a request to edit anomalies', async () => {
      const anomalies = [dummyAnomaly(0)];
      const traceNames = ['trace1'];
      fetchMock.post('/_/triage/edit_anomalies', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          keys: [Number(anomalies[0].id)],
          trace_names: traceNames,
          action: 'IGNORE',
        });
        return { status: 200, body: JSON.stringify({}) };
      });

      element.addEventListener('anomaly-changed', (e) => {
        assert.equal((e as CustomEvent).detail.editAction, 'IGNORE');
      });

      await element.makeEditAnomalyRequest(anomalies, traceNames, 'IGNORE');
      await fetchMock.flush(true);
      // Assert that the toast dialog pops up.
      assert.isNotNull(element.ignoreTriageToast);
    });
  });

  describe('make nudge request', () => {
    it('sends a request to nudge anomalies', async () => {
      const anomalies = [dummyAnomaly(0)];
      const traceNames = ['trace1'];
      const entry = new NudgeEntry();
      entry.start_revision = 123;
      entry.end_revision = 456;
      fetchMock.post('/_/triage/edit_anomalies', (_url, opts) => {
        const body = JSON.parse(opts.body as string);
        assert.deepEqual(body, {
          keys: [anomalies[0].id],
          trace_names: traceNames,
          action: 'NUDGE',
          start_revision: entry.start_revision,
          end_revision: entry.end_revision,
        });
        return { status: 200, body: JSON.stringify({}) };
      });

      element.addEventListener('anomaly-changed', (e) => {
        assert.deepEqual((e as CustomEvent).detail.anomalies, [entry.anomaly_data?.anomaly]);
      });

      element.makeNudgeRequest(anomalies, traceNames, entry);
      await fetchMock.flush(true);
    });

    it('dispatches anomaly-changed event with correct detail', async () => {
      const anomalies = [dummyAnomaly(0)];
      const traceNames = ['trace1'];
      const entry = new NudgeEntry();
      entry.start_revision = 123;
      entry.end_revision = 456;
      entry.anomaly_data = {
        anomaly: anomalies[0],
        x: 0,
        y: 0,
        highlight: true,
      };
      fetchMock.post('/_/triage/edit_anomalies', { status: 200, body: JSON.stringify({}) });

      await element.makeNudgeRequest(anomalies, traceNames, entry);
      fetchMock.done();
      await fetchMock.flush(true);

      element.addEventListener('anomaly-changed', (e) => {
        assert.deepEqual((e as CustomEvent).detail.traceNames, traceNames);
        assert.deepEqual((e as CustomEvent).detail.displayIndex, 0);
        assert.deepEqual((e as CustomEvent).detail.anomalies, [entry.anomaly_data?.anomaly]);
      });
    });
  });
});
