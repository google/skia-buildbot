import './index';

document.querySelector('triage-menu-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
