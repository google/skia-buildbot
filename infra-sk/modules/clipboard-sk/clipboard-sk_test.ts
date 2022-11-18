import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { ClipboardSk, defaultToolTipMessage } from './clipboard-sk';

import { setUpElementUnderTest } from '../test_util';
import { TooltipSk } from '../tooltip-sk/tooltip-sk';

const testMessage = 'This should end up in the clipboard';

describe('clipboard-sk', () => {
  const newInstance = setUpElementUnderTest<ClipboardSk>('clipboard-sk');

  let element: ClipboardSk;
  beforeEach(() => {
    element = newInstance((el: ClipboardSk) => {
      el.value = testMessage;
    });
  });

  describe('on construction', () => {
    it('has the right tooltip value', () => {
      assert.equal($$<TooltipSk>('tooltip-sk', element)!.value, defaultToolTipMessage);
    });
  });
});
