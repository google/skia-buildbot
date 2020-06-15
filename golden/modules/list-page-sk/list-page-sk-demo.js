import './index';

document.querySelector('list-page-sk').addEventListener('some-event-name', (e) => {
  document.querySelector('#events').textContent = JSON.stringify(e.detail, null, '  ');
});
