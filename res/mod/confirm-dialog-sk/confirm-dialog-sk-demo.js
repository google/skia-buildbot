import './index.js'
import 'skia-elements/buttons'
import { $ } from 'skia-elements/dom'

$('ask').addEventListener('click', e => {
  $('dialog').open('Do something dangerous?').then(() => {
    $('results').textContent = 'Confirmed!';
  }).catch(() => {
    $('results').textContent = 'Cancelled!';
  });
})
