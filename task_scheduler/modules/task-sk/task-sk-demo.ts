import './index';

document
  .querySelector('task-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  '
    );
  });
