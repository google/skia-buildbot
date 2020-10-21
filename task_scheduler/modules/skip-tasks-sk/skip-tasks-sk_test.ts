import './index';
import { SkipTasksSk } from './skip-tasks-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('skip-tasks-sk', () => {
  const newInstance = setUpElementUnderTest<SkipTasksSk>('skip-tasks-sk');

  let element: SkipTasksSk;
  beforeEach(() => {
    element = newInstance((el: SkipTasksSk) => {
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
