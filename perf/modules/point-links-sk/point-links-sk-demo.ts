import fetchMock from 'fetch-mock';
import { CommitNumber } from '../json';
import './index';
import { PointLinksSk } from './point-links-sk';

fetchMock.post('/_/details/?results=false', (_url, request) => {
  const requestObj = JSON.parse(request.body!.toString());
  switch (requestObj.cid) {
    case 12:
      return {
        version: 1,
        links: {
          'V8 Git Hash':
            'https://chromium.googlesource.com/v8/v8/+/47f420e89ec1b33dacc048d93e0317ab7fec43dd',
        },
      };
    case 11:
      return {
        version: 1,
        links: {
          'V8 Git Hash':
            'https://chromium.googlesource.com/v8/v8/+/f052b8c4db1f08d1f8275351c047854e6ff1805f',
        },
      };
    case 10:
      return {
        version: 1,
        links: {
          'V8 Git Hash':
            'https://chromium.googlesource.com/v8/v8/+/f052b8c4db1f08d1f8275351c047854e6ff1805f',
        },
      };
    default:
      return {};
  }
});

const returnLinks = {
  buildKey: 'https://v8/builder/build1',
  traceKey: 'https://traceViewer/trace',
};

fetchMock.post('/_/links/', {
  version: 1,
  links: returnLinks,
});

// Expose fetchMock globally and ensure it intercepts fetch calls.
// This is crucial for Puppeteer tests where fetchMock needs to be active on the page.
(window as any).fetchMock = fetchMock;

window.customElements.whenDefined('point-links-sk').then(() => {
  const links1 = document.getElementById('different-commits') as PointLinksSk;
  links1.load(CommitNumber(12), CommitNumber(11), 'foo', ['V8 Git Hash'], ['Build Page'], []);
});
