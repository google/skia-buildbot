import { TestSrcSk } from './test-src-sk';
import './test-src-sk';
import fetchMock from 'fetch-mock';
import {
  setUpElementUnderTest,
  eventPromise,
} from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';

fetchMock.config.overwriteRoutes = true;

describe('test-src-sk', () => {
  const newInstance = setUpElementUnderTest<TestSrcSk>('test-src-sk');

  let element: TestSrcSk;
  beforeEach(() => {
    element = newInstance((el: TestSrcSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', async () => {
      const value = 'Hello world!';
      const url = '/some-text-endpoint';
      fetchMock.get(url, value);
      element.src = url;
      await fetchMock.flush(true);
      assert.equal(element.querySelector('pre')!.innerText, value);
    });
  });
});
