import './index';

document
  .querySelector('scrap-exchange-sk')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  ',
    );
  });
