import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/error-toast-sk';
import { AnomaliesTableSk } from './anomalies-table-sk';
import fetchMock from 'fetch-mock';
import {
  anomaly_table,
  anomaly_table_for_grouping,
  anomaly_table_for_tooltip,
  GROUP_REPORT_RESPONSE,
  GROUP_REPORT_RESPONSE_WITH_SID,
  mockAssociatedIssues,
} from './test_data';

async function delay(time: number) {
  return await new Promise((resolve) => setTimeout(resolve, time));
}

fetchMock.post('/_/triage/file_bug', async () => {
  await delay(1000);
  return {
    bug_id: 358011161,
  };
});

fetchMock.post('/_/triage/list_issues', async () => {
  return {
    body: JSON.stringify({
      issues: mockAssociatedIssues,
    }),
  };
});

fetchMock.post('/_/triage/associate_alerts', async () => {
  delay(1000);
  return {
    bug_id: 474535097,
  };
});

window.perf = {
  dev_mode: false,
  instance_url: '',
  instance_name: 'chrome-perf-demo',
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
  bug_host_url: 'http://bug_host',
  git_repo_url: '',
  keys_for_commit_range: [],
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
  remove_default_stat_value: false,
  enable_skia_bridge_aggregation: false,
  show_json_file_display: false,
  always_show_commit_info: false,
  show_triage_link: false,
  show_bisect_btn: false,
  app_version: 'test-version',
  enable_v2_ui: false,
  extra_links: null,
};

$$('#populate-tables')?.addEventListener('click', () => {
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.populateTable(anomaly_table);
  });
});

$$('#populate-tables-for-grouping')?.addEventListener('click', () => {
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.populateTable(anomaly_table_for_grouping);
  });
});

$$('#populate-tables-it')?.addEventListener('click', async () => {
  // The anomly_list in the response is what is passed to the anomalies table.
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.populateTable([anomaly_table_for_tooltip[0]]);
  });
});

$$('#populate-tables-it1')?.addEventListener('click', async () => {
  // The anomly_list in the response is what is passed to the anomalies table.
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.populateTable([anomaly_table_for_tooltip[1]]);
  });
});

$$('#populate-tables-it2')?.addEventListener('click', async () => {
  // The anomly_list in the response is what is passed to the anomalies table.
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.populateTable([anomaly_table_for_tooltip[2]]);
  });
});

$$('#open-multi-graph')?.addEventListener('click', () => {
  document.querySelectorAll<AnomaliesTableSk>('anomalies-table-sk').forEach((table) => {
    table.shortcutUrl = `test_shortcut`;
    table.openMultiGraphUrl(
      anomaly_table[0],
      window.open(
        'http://localhost:46723/m/?begin=1729042589&end=11739042589&request_type=0&shortcut=1&totalGraphs=1',
        '_blank'
      )
    );
  });
});

fetchMock.post('/_/shortcut/update', () => ({
  id: 'test_shortcut',
}));

// Generic mock for group_report that inspects body
fetchMock.post('/_/anomalies/group_report', (_url, options) => {
  if (options.body) {
    const body = typeof options.body === 'string' ? JSON.parse(options.body) : options.body;
    if (body.anomalyIDs === '1,2,3' || body.anomalyIDs === '1,2') {
      return GROUP_REPORT_RESPONSE_WITH_SID;
    }
  }
  return GROUP_REPORT_RESPONSE;
});
