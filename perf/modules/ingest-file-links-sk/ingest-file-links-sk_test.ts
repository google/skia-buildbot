import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { IngestFileLinksSk } from './ingest-file-links-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CommitNumber } from '../json';

describe('ingest-file-links-sk', () => {
  const validCommitID = CommitNumber(12);
  const validTraceID = ',arch=x86,';
  const newInstance = setUpElementUnderTest<IngestFileLinksSk>('ingest-file-links-sk');

  let element: IngestFileLinksSk;
  beforeEach(() => {
    element = newInstance();
    fetchMock.reset();
  });

  describe('load', () => {
    it('requires valid parameters', async () => {
      await element.load(CommitNumber(-1), '');
      assert.isEmpty($<HTMLLinkElement>('a', element));
    });

    it('displays each link in sorted order', async () => {
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
      await element.load(validCommitID, validTraceID);
      const linkElements = $<HTMLLinkElement>('a', element);
      assert.equal(2, linkElements.length);
      assert.include(linkElements[0].textContent, 'Perfetto Results');
      assert.include(linkElements[1].textContent, 'Swarming Run');
      assert.include(linkElements[0].href, 'https://skia.org');
      assert.include(linkElements[1].href, 'https://skia.org');

      const listElements = $<HTMLLIElement>('li', element);
      assert.equal(5, listElements.length);
      assert.include(listElements[0].textContent, 'Bot Id: build109-h7,build109-h8');
      assert.include(listElements[1].textContent, 'Foo: /bar');
      assert.include(listElements[2].textContent, 'Go Link: go/skia');
      assert.include(listElements[3].textContent, 'Perfetto Results');
      assert.include(listElements[4].textContent, 'Swarming Run');
    });

    it('stops spinning on fetch error', async () => {
      fetchMock.post('/_/details/?results=false', 500);
      await element.load(validCommitID, validTraceID);
      assert.isEmpty($<HTMLLinkElement>('a', element));
      assert.isFalse($$<SpinnerSk>('spinner-sk')?.active);
    });

    it('does not display links if version is missing', async () => {
      fetchMock.post('/_/details/?results=false', () => ({
        // version: null,
        links: {
          'Swarming Run': 'https://skia.org',
          'Perfetto Results': 'https://skia.org',
        },
      }));
      await element.load(validCommitID, validTraceID);
      assert.isEmpty($<HTMLLinkElement>('a', element));
    });

    it('markdown links are filtered to url and regular urls are unaffected', async () => {
      fetchMock.post('/_/details/?results=false', () => ({
        version: 1,
        links: {
          'Benchmark Config': '[Benchmark Config](https://skia.org)',
          'Perfetto Results': 'https://perfetto.results.org',
        },
      }));
      await element.load(validCommitID, validTraceID);
      const linkElements = $<HTMLLinkElement>('a', element);
      assert.equal(2, linkElements.length);
      assert.equal(linkElements[0].textContent, 'Benchmark Config');
      assert.equal(linkElements[1].textContent, 'Perfetto Results');
      assert.equal(linkElements[0].href, 'https://skia.org/');
      assert.equal(linkElements[1].href, 'https://perfetto.results.org/');
    });
  });
});
