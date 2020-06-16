import './index';
import '../@bundled-es-modules/fetch-mock'; // @bundled-es-modules/fetch-mock doesn't have typings.
import { fetchMock } from '@bundled-es-modules/fetch-mock';
import { JSONSourceSk } from './json-source-sk';
import 'elements-sk/error-toast-sk';

fetchMock.post('/_/details/', () => {
  return { Hello: 'world!' };
});

window.customElements.whenDefined('json-source-sk').then(() => {
  const sources = document.querySelectorAll<JSONSourceSk>('json-source-sk')!;
  sources.forEach((source) => {
    source.traceid = 'foo';
    source.cid = {
      offset: 12,
    };
    source.querySelector<HTMLButtonElement>('#controls button')!.click();
  });
});
