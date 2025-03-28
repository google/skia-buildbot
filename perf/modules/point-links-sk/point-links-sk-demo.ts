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

window.customElements.whenDefined('point-links-sk').then(() => {
  const links1 = document.getElementById('different-commits') as PointLinksSk;
  const links2 = document.getElementById('same-commits') as PointLinksSk;
  links1.load(CommitNumber(12), CommitNumber(11), 'foo', ['V8 Git Hash'], ['Build Page'], []);
  links2.load(CommitNumber(10), CommitNumber(11), 'foo', ['V8 Git Hash'], ['Build Page'], []);
});
