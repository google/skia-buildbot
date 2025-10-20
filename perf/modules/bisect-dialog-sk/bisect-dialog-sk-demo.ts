import './index';
import fetchMock from 'fetch-mock';
import { BisectDialogSk } from './bisect-dialog-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/error-toast-sk';
import { anomalies } from '../existing-bug-dialog-sk/test_data';

function delay(time: number) {
  return new Promise((resolve) => setTimeout(resolve, time));
}

fetchMock.post('/_/login/status', async () => {
  await delay(2000);
  return {
    email: 'someone@example.org',
    roles: ['editor'],
  };
});

fetchMock.post('/_/triage/file_bug', async () => {
  await delay(2000);
  return {
    bug_id: 358011161,
  };
});

fetchMock.post('/_/bisect/create', async () => {
  await delay(2000);
  return {
    jobId: '12345',
    jobUrl: '/job/1',
  };
});

window.customElements.whenDefined('bisect-dialog-sk').then(() => {
  const ele = document.querySelector('bisect-dialog-sk') as BisectDialogSk;

  ele.setBisectInputParams({
    testPath: anomalies[0].test_path,
    startCommit: anomalies[0].start_revision.toString(),
    endCommit: anomalies[0].end_revision.toString(),
    bugId: anomalies[0].bug_id.toString(),
    story: 'story',
    anomalyId: anomalies[0].id,
  });

  ele.addEventListener('anomaly-changed', (e) => {
    const detail = (e as CustomEvent).detail;
    const eventContents = $$('#events')!;
    eventContents.textContent = JSON.stringify(detail, null, '  ');
  });
});

$$('#show-dialog')?.addEventListener('click', () => {
  const ele = document.querySelector('bisect-dialog-sk') as BisectDialogSk;
  ele.open();
});
