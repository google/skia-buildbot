import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { AnomaliesTableSk } from './anomalies-table-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly } from '../json';
import sinon from 'sinon';

describe('anomalies-table-sk', () => {
  const newInstance = setUpElementUnderTest<AnomaliesTableSk>('anomalies-table-sk');

  let element: AnomaliesTableSk;
  beforeEach(() => {
    window.perf = {
      instance_url: '',
      commit_range_url: '',
      key_order: [],
      demo: false,
      radius: 0,
      num_shift: 0,
      interesting: 0,
      step_up_only: false,
      display_group_by: false,
      hide_list_of_commits_on_explore: false,
      notifications: 'none',
      fetch_chrome_perf_anomalies: false,
      feedback_url: '',
      chat_url: '',
      help_url_override: '',
      trace_format: 'chrome',
      need_alert_action: false,
      bug_host_url: '',
      git_repo_url: '',
      keys_for_commit_range: [],
      keys_for_useful_links: [],
      skip_commit_detail_display: false,
      image_tag: '',
      remove_default_stat_value: false,
      enable_skia_bridge_aggregation: false,
      show_json_file_display: false,
      always_show_commit_info: false,
      show_triage_link: false,
    };
    element = newInstance((el: AnomaliesTableSk) => {
      el.populateTable([], {});
    });
  });

  afterEach(() => {
    fetchMock.reset();
    sinon.restore();
  });

  describe('openMultiGraphUrl', () => {
    const anomaly: Anomaly = {
      id: '123',
      test_path: 'master/bot/test/subtest',
      bug_id: 456,
      start_revision: 100,
      end_revision: 200,
      is_improvement: false,
      recovered: false,
      state: 'new',
      statistic: 'mean',
      units: 'ms',
      degrees_of_freedom: 1,
      median_before_anomaly: 10,
      median_after_anomaly: 20,
      p_value: 0.01,
      segment_size_after: 10,
      segment_size_before: 10,
      std_dev_before_anomaly: 1,
      t_statistic: 2,
      subscription_name: 'sub',
      bug_component: '',
      bug_labels: [],
      bug_cc_emails: [],
      bisect_ids: [],
    };

    beforeEach(() => {
      fetchMock.post('begin:/_/anomalies/group_report', {
        sid: 'test-sid',
        timerange_map: {
          '123': {
            begin: 100,
            end: 200,
          },
        },
      });
      fetchMock.post('begin:/_/shortcut/update', {
        id: 'test-shortcut',
      });
    });

    it('opens a new window with the correct URL', async () => {
      // Stub window.open to capture the URL.
      let openedUrl = '';
      window.open = (url?: string | URL, _target?: string, _features?: string) => {
        openedUrl = url as string;
        return null;
      };

      await element.populateTable([anomaly], {
        '123': {
          begin: 100,
          end: 200,
        },
      });
      await element.openMultiGraphUrl(anomaly);

      expect(openedUrl).to.contain('/m/');
      expect(openedUrl).to.contain('shortcut=test-shortcut');
    });

    it('is called when open-anomaly-chart event is dispatched', async () => {
      const openMultiGraphUrlStub = sinon.stub(element, 'openMultiGraphUrl');

      const event = new CustomEvent('open-anomaly-chart', {
        detail: anomaly,
        bubbles: true,
      });
      element.dispatchEvent(event);

      expect(openMultiGraphUrlStub.calledOnceWith(anomaly)).to.equal(true);
    });
  });
});
