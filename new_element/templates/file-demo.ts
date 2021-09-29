import './index';

document
  .querySelector('{{.ElementName}}')!
  .addEventListener('some-event-name', (e) => {
    document.querySelector('#events')!.textContent = JSON.stringify(
      e,
      null,
      '  ',
    );
  });
