import fetchMock from 'fetch-mock';
import './index';
import { IngestFileLinksSk } from './ingest-file-links-sk';
import { CommitNumber } from '../json';

fetchMock.post('/_/details/?results=false', () => ({
  version: 1,
  links: {
    'Swarming Run': 'https://skia.org',
    'Perfetto Results': 'https://skia.org',
    'Bot Id': 'build109-h7,build109-h8',
    Foo: '/bar',
    'Go Link': 'go/skia',
  },
}));

window.customElements.whenDefined('ingest-file-links-sk').then(() => {
  const sources = document.querySelectorAll<IngestFileLinksSk>(
    'ingest-file-links-sk'
  )!;
  sources.forEach((source) => {
    source.load(CommitNumber(12), 'foo');
  });
});
