import './index';
import { UniformTimeSk } from './uniform-time-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';

describe('uniform-time-sk', () => {
  const newInstance = setUpElementUnderTest<UniformTimeSk>('uniform-time-sk');

  let element: UniformTimeSk;
  beforeEach(() => {
    element = newInstance((el: UniformTimeSk) => {
      el.dateNow = () => 0;
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('throws on invalid uniform', () => {
      assert.throws(() => {
        element.uniform = {
          name: 'iTime',
          rows: 2,
          columns: 2,
          slot: 1,
        };
      });
    });
  });
});
