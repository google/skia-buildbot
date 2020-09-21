import './index';

document
  .querySelector('algo-select-sk')!
  .addEventListener('algo-change', (e) => {
    document.querySelector('pre')!.textContent = JSON.stringify(
      (e as CustomEvent).detail,
      null,
      '',
    );
  });
