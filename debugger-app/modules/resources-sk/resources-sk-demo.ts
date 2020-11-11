import './index';

document
  .querySelector('resources-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  '
    );
  });
