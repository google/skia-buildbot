import { TestSrc } from './test-src';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('test-src', () => {
  const newInstance = setUpElementUnderTest<TestSrc>('test-src');

  let element: TestSrc;
  beforeEach(() => {
    element = newInstance((el: TestSrc) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});


