import './index';
import { ResourcesSk } from './resources-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('resources-sk', () => {
  const newInstance = setUpElementUnderTest<ResourcesSk>('resources-sk');

  let element: ResourcesSk;
  beforeEach(() => {
    element = newInstance((el: ResourcesSk) => {
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
