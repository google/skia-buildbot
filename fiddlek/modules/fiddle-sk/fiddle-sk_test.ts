import { FiddleSk } from './fiddle-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('fiddle-sk', () => {
  const newInstance = setUpElementUnderTest<FiddleSk>('fiddle-sk');

  let element: FiddleSk;
  beforeEach(() => {
    element = newInstance((el: FiddleSk) => {
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
