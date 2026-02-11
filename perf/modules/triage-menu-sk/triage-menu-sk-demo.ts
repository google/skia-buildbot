import './index';
import fetchMock from 'fetch-mock';
import { NudgeEntry } from './triage-menu-sk';
import { TriageMenuSk } from './triage-menu-sk';
import { anomaly_table } from '../anomalies-table-sk/test_data';

fetchMock.post('/_/triage/edit_anomalies', () => {
  return {};
});

fetchMock.post('/_/triage/file_bug', () => {
  return { bug_id: 12345 };
});

fetchMock.post('/_/bug/associate', () => {
  return {};
});

export const mockNudgeList: NudgeEntry[] = [
  {
    selected: false,
    anomaly_data: null,
    start_revision: 50,
    end_revision: 55,
    display_index: 0,
    x: 0,
    y: 0,
  },
  {
    selected: true,
    anomaly_data: null,
    start_revision: 55,
    end_revision: 60,
    display_index: 1,
    x: 10,
    y: 10,
  },
];

const triageMenuSk = document.querySelector('triage-menu-sk') as TriageMenuSk;
if (triageMenuSk) {
  triageMenuSk.setAnomalies(
    anomaly_table,
    ['ChromiumPerf/mac-m1_mini_2020-perf/jetstream2/stanford-crypto-aes.Average/JetStream2'],
    mockNudgeList
  );
}
