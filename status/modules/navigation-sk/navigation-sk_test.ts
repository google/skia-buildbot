import './index';
import { expect } from 'chai';
import { $ } from 'common-sk/modules/dom';
import { NavigationSk } from './navigation-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

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
