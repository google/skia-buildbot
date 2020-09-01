import './index';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('{{.ElementName}}', () => {
  const newInstance = setUpElementUnderTest('{{.ElementName}}');

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
