import './index';

document.querySelector('bulk-triage-sk').addEventListener('some-event-name', (e) => {
  document.querySelector('#events').textContent = JSON.stringify(e.detail, null, '  ');
});
