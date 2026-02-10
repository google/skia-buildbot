import './index';
import fetchMock from 'fetch-mock';
import { ExistingBugDialogSk } from './existing-bug-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/error-toast-sk';
import { anomalies } from './test_data';

fetchMock.get('/_/login/status', {
  email: 'someone@example.org',
  roles: ['editor'],
});

async function delay(time: number) {
  return await new Promise((resolve) => setTimeout(resolve, time));
}

fetchMock.post('/_/triage/associate_alerts', async () => {
  await delay(2000);
  return {
    bug_id: 358011161,
  };
});

fetchMock.post('/_/anomalies/group_report', async () => {
  return {
    anomalyList: anomalies,
  };
});

window.customElements.whenDefined('existing-bug-dialog-sk').then(() => {
  const ele = document.querySelector('existing-bug-dialog-sk') as ExistingBugDialogSk;
  ele.anomalies = anomalies;
  ele.traceNames = [];
});

$$('#demo-open')?.addEventListener('click', () => {
  const ele = document.querySelector('existing-bug-dialog-sk') as ExistingBugDialogSk;
  ele.open();
});
