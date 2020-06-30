import './index';

document.querySelector('cluster-digests-sk').addEventListener('some-event-name', (e) => {
  document.querySelector('#events').textContent = JSON.stringify(e.detail, null, '  ');
});
