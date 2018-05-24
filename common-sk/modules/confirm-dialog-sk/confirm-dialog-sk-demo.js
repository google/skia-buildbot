import 'elements-sk/buttons'

import { $$ } from 'common/dom'

import './index.js'

$$('#ask').addEventListener('click', e => {
  $$('#dialog').open('Do something dangerous?').then(() => {
    $$('#results').textContent = 'Confirmed!';
  }).catch(() => {
    $$('#results').textContent = 'Cancelled!';
  });
})
