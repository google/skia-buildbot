import './index';
import '../../../elements-sk/modules/error-toast-sk';
import fetchMock from 'fetch-mock';
import { AnomaliesTableRow, AnomaliesTableColumn } from '../anomalies-table-sk';

// window.perf = window.perf || {};
// window.perf.key_order = [];
// window.perf.display_group_by = true;
// window.perf.notifications = 'markdown_issuetracker';

fetchMock.get('/_/login/status', {
  email: 'someone@example.org',
  roles: ['editor'],
});

function delay(time: number) {
  return new Promise((resolve) => setTimeout(resolve, time));
}

// Python backend (reference): /chromium/src/third_party/catapult/dashboard/dashboard/alerts.py
// Backend proxy (use): https://skia-review.googlesource.com/c/buildbot/+/912874
fetchMock.post('/_/alerts/get_anomaly_list', async () => {
  await delay(2000);

  console.log('mock backend get_anomaly_list');

  const column: AnomaliesTableColumn = {
    check_header: false,
    graph_header: null,
    bug_id: '12345',
    end_revision: 'endrevise',
    master: 'master',
    bot: 'bot',
    test_suite: 'test_suite',
    test: 'test',
    change_direction: 'changedirection',
    percent_changed: 'percentChanged',
    absolute_delta: 'absoluteDelta',
  };

  const row: AnomaliesTableRow = {
    columns: [column],
  };

  return {
    header: [],
    table: [row],
  };
});

document.querySelector('#anomalies')!.innerHTML = '<anomalies-table-sk></anomalies-table-sk>';
