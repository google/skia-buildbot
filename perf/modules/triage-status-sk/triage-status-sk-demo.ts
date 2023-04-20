import './index';
import { TriageStatusSk } from './triage-status-sk';

document
  .querySelector<TriageStatusSk>('triage-status-sk')!
  .addEventListener('start-triage', (e) => {
    document.querySelector('pre')!.textContent = JSON.stringify(
      (e as CustomEvent).detail,
      null,
      '  '
    );
  });

document.querySelectorAll<TriageStatusSk>('triage-status-sk').forEach((e) => {
  e.triage = {
    status: 'negative',
    message: '',
  };
});
