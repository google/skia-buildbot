import './index';
import { NavigationSk } from './navigation-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { $ } from 'common-sk/modules/dom';

describe('navigation-sk', () => {
  const newInstance = setUpElementUnderTest<NavigationSk>('navigation-sk');

  let element: NavigationSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('display', () => {
    it('navigation items', () => {
      expect($('.tr', element)).to.have.length(2);
    });
  });
});
