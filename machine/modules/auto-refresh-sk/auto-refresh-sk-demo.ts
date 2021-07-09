import { time } from 'node:console';
import './index';

document
  .querySelector('auto-refresh-sk')!
  .addEventListener('refresh-page', (e) => {
    document.querySelector('#events')!.textContent = `${JSON.stringify(
      e,
      null,
      '  ',
    )}\n${Date.now()}`;
  });
