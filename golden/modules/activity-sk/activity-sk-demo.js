import './index.js'

import { $$ } from 'common-sk/modules/dom'

const el = $$('activity-sk');
const text = 'Hello, world!';

el.startSpinner(text);

$$('#toggle').addEventListener('click', function() {
  if (el.isSpinning) {
    el.stopSpinner();
  } else {
    el.startSpinner(text);
  }
});
