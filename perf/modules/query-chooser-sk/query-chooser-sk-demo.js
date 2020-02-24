import './index';
import { fetchMock } from '@bundled-es-modules/fetch-mock';

// Wait 1s before returning the content so we can see the spinner in action.
fetchMock.post('/', async () => new Promise((res) => setTimeout(() => res({ count: Math.floor(Math.random() * 2000) }), 1000)));
