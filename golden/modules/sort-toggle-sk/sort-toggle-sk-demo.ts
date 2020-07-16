import './index';

document.querySelector('sort-toggle-sk').addEventListener('some-event-name', (e) => {
  document.querySelector('#events').textContent = JSON.stringify(e.detail, null, '  ');
});
