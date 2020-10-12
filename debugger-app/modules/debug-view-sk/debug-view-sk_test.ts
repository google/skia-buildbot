import './index';
import { DebugViewSk } from './debug-view-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('debug-view-sk', () => {
  const newInstance = setUpElementUnderTest<DebugViewSk>('debug-view-sk');

  let element: DebugViewSk;
  beforeEach(() => {
    element = newInstance((el: DebugViewSk) => {
    });
  });

  // TODO(nifong): add tests after implementing crosshair
});
