import './index';
import fetchMock from 'fetch-mock';
import { JSONSourceSk } from './json-source-sk';
import 'elements-sk/error-toast-sk';

fetchMock.post('/_/details/', () => {
  return { Hello: 'world!' };
});

window.customElements.whenDefined('json-source-sk').then(() => {
  const sources = document.querySelectorAll<JSONSourceSk>('json-source-sk')!;
  sources.forEach((source) => {
    source.traceid = 'foo';
    source.cid = 12;
    source.querySelector<HTMLButtonElement>('#controls button')!.click();
  });
});
