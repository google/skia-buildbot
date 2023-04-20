import './index';

document.querySelectorAll('day-range-sk').forEach((ele) => {
  ele.addEventListener('day-range-change', (e) => {
    document.getElementById('event')!.textContent = JSON.stringify(
      (e as CustomEvent).detail,
      null,
      ' '
    );
  });
});
