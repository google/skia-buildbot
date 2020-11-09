import './index';

document
  .querySelector('android-layers-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  '
    );
  });
