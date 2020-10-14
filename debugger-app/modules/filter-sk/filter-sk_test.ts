import './index';
import { FilterSk } from './filter-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('filter-sk', () => {
  const newInstance = setUpElementUnderTest<FilterSk>('filter-sk');

  let element: FilterSk;
  beforeEach(() => {
    element = newInstance((el: FilterSk) => {
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
