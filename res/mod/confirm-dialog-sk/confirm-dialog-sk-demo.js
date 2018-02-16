import './index.js'
import 'skia-elements/buttons'
import { $$ } from 'common/dom'

$$('#ask').addEventListener('click', e => {
  $$('#dialog').open('Do something dangerous?').then(() => {
    $$('#results').textContent = 'Confirmed!';
  }).catch(() => {
    $$('#results').textContent = 'Cancelled!';
  });
})
