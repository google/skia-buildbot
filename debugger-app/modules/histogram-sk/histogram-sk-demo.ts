import './index';

document
  .querySelector('histogram-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  ',
    );
  });
