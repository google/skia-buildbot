import './index';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('example-control-sk', () => {
  const newInstance = setUpElementUnderTest('example-control-sk');

  let element;
  beforeEach(() => {
    element = newInstance((el) => {
      // Come to run every time the instance is created.
    });
  });

  describe('some action', () => {
    it('some result', () => {
    });
  });
});
