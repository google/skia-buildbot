import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from 'common-sk/modules/dom';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { IngestFileLinksSk } from './ingest-file-links-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('ingest-file-links-sk', () => {
  const validCommitID = 12;
  const validTraceID = ',arch=x86,';
  const newInstance = setUpElementUnderTest<IngestFileLinksSk>('ingest-file-links-sk');

  let element: IngestFileLinksSk;
  beforeEach(() => {
    element = newInstance();
    fetchMock.reset();
  });

  describe('load', () => {
    it('requires valid parameters', async () => {
      await element.load(-1, '');
      assert.isEmpty($<HTMLLinkElement>('a', element));
    });

    it('displays each link in sorted order', async () => {
      fetchMock.post('/_/details/?results=false', () => ({
        version: 1,
        links: {
          'Swarming Run': 'https://skia.org',
          'Perfetto Results': 'https://skia.org',
        },
      }));
      await element.load(validCommitID, validTraceID);
      const linkElements = $<HTMLLinkElement>('a', element);
      assert.equal(2, linkElements.length);
      assert.include(linkElements[0].textContent, 'Perfetto Results');
      assert.include(linkElements[1].textContent, 'Swarming Run');
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
  });
});
