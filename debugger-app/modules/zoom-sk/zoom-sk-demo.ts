import './index';

document
  .querySelector('zoom-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  ',
    );
  });
