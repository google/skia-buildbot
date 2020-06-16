import './index';
import '../@bundled-es-modules/fetch-mock'; // @bundled-es-modules/fetch-mock doesn't have typings.
import { fetchMock } from '@bundled-es-modules/fetch-mock';
import { QueryCountSk } from './query-count-sk';
import 'elements-sk/error-toast-sk';

let count = 11;

fetchMock.post('/', () => {
  return { count: count };
});

window.customElements.whenDefined('query-count-sk').then(() => {
  const qcs = document.querySelectorAll<QueryCountSk>('query-count-sk')!;
  qcs.forEach((qc) => {
    qc.url = '/';
    document
      .querySelector<HTMLButtonElement>('#make_query')!
      .addEventListener('click', (e) => {
        count += 1;
        qc.current_query = 'config=565';
      });
  });
});
