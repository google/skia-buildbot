import './index';
import { TimelineSk } from './timeline-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('timeline-sk', () => {
  const newInstance = setUpElementUnderTest<TimelineSk>('timeline-sk');

  let element: TimelineSk;
  beforeEach(() => {
    element = newInstance((el: TimelineSk) => {
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
