/* eslint-disable dot-notation */
import { assert } from 'chai';
import { HResizableBoxSk } from './h_resizable_box_sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

import './h_resizable_box_sk';

describe('h-resizable-box-sk', () => {
  const newEl = setUpElementUnderTest<HResizableBoxSk>('h-resizable-box-sk');

  // Karma don't render and the style doesn't work the same way when running
  // in a browser. We only do a smoke check on its creation and existance.
  // More tests should be done in puppeteer and integration tests.
  it('init test', async () => {
    const el = newEl();
    assert.equal(el.nodeName.toLowerCase(), 'h-resizable-box-sk');
  });
});
