import './index';

document
  .querySelector('play-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  '
    );
  });
