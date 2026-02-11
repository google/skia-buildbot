import './index';
import fetchMock from 'fetch-mock';
import { Anomaly } from '../json';
import { NewBugDialogSk } from './new-bug-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/error-toast-sk';

fetchMock.get('/_/login/status', {
  email: 'someone@example.org',
  roles: ['editor'],
});

function delay(time: number) {
  return new Promise((resolve) => setTimeout(resolve, time));
}

fetchMock.post('/_/triage/file_bug', async () => {
  await delay(2000);
  return {
    bug_id: 358011161,
  };
});

window.customElements.whenDefined('new-bug-dialog-sk').then(() => {
  const ele = document.querySelector('new-bug-dialog-sk') as NewBugDialogSk;

  const anomalies: Anomaly[] = [
    {
      id: '1',
      test_path: 'internal.client.v8/x64/v8/JetStream2/maglev-future/async-fs/Average',
      bug_id: 0,
      start_revision: 95942,
      end_revision: 95942,
      is_improvement: false,
      recovered: false,
      state: '',
      statistic: '',
      units: 'score',
      degrees_of_freedom: 0,
      median_before_anomaly: 108.074,
      median_after_anomaly: 102.443,
      p_value: 0,
      segment_size_after: 0,
      segment_size_before: 0,
      std_dev_before_anomaly: 0,
      t_statistic: 0,
      subscription_name: 'Dummy Perf Sheriff',
      bug_component: 'ComponentA>SubComponentA',
      bug_labels: ['Label1', 'Label2'],
      bug_cc_emails: ['abcd@google.com'],
      bisect_ids: [],
    },
    {
      id: '2',
      test_path: 'internal.client.v8/x64/v8/JetStream2/maglev-future/async-fs/Wall-Time',
      bug_id: 0,
      start_revision: 95940,
      end_revision: 95944,
      is_improvement: false,
      recovered: false,
      state: '',
      statistic: '',
      units: '',
      degrees_of_freedom: 0,
      median_before_anomaly: 1854.3049999999998,
      median_after_anomaly: 1953.7269999999999,
      p_value: 0,
      segment_size_after: 0,
      segment_size_before: 0,
      std_dev_before_anomaly: 0,
      t_statistic: 0,
      subscription_name: 'Dummy Perf Sheriff',
      bug_component: 'ComponentB>SubComponentB>SubcomponentC',
      bug_labels: ['Label1', 'Label2'],
      bug_cc_emails: ['abcd@google.com'],
      bisect_ids: [],
    },
    {
      id: '3',
      test_path: 'internal.client.v8/x64/v8/JetStream2/maglev/async-fs/Average-Score',
      bug_id: 0,
      start_revision: 95944,
      end_revision: 95945,
      is_improvement: false,
      recovered: false,
      state: '',
      statistic: '',
      units: '',
      degrees_of_freedom: 0,
      median_before_anomaly: 46.2635,
      median_after_anomaly: 48.7535,
      p_value: 0,
      segment_size_after: 0,
      segment_size_before: 0,
      std_dev_before_anomaly: 0,
      t_statistic: 0,
      subscription_name: 'Dummy Perf Sheriff',
      bug_component: 'ComponentB>SubComponentB>SubcomponentC',
      bug_labels: ['Label1', 'Label3'],
      bug_cc_emails: ['abcd@google.com'],
      bisect_ids: [],
    },
  ];

  ele.anomalies = anomalies;
  ele.traceNames = [''];
});

$$('#file-bug')?.addEventListener('click', () => {
  const ele = document.querySelector('new-bug-dialog-sk') as NewBugDialogSk;
  ele.fileNewBug();
});
