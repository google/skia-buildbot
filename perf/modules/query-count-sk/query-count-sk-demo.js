import './index.js'
import { fetchMock } from '@bundled-es-modules/fetch-mock';

fetchMock.post('/', async function() {
  return await new Promise(res => setTimeout(() => res({count: Math.floor(Math.random()*2000)}), 1000))
});

