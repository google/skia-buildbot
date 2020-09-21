import './index';
import fetchMock from 'fetch-mock';
import { QueryCountSk } from './query-count-sk';
import 'elements-sk/error-toast-sk';

let count = 11;

fetchMock.post('/', () => ({ count: count }));

window.customElements.whenDefined('query-count-sk').then(() => {
  const qcs = document.querySelectorAll<QueryCountSk>('query-count-sk')!;
  qcs.forEach((qc) => {
    qc.url = '/';
    document
      .querySelector<HTMLButtonElement>('#make_query')!
      .addEventListener('click', () => {
        count += 1;
        qc.current_query = 'config=565';
      });
  });
});
