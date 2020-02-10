import './index';
import { fetchMock } from '@bundled-es-modules/fetch-mock';

fetchMock.post('/',
  // Wait 1s before returning the content so we can see the spinner in action.
  async () => new Promise((res) => setTimeout(() => res({ count: Math.floor(Math.random() * 2000) }), 1000)));
