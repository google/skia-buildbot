import './index.js'
import 'skia-elements/buttons'
import { $ } from 'skia-elements/core'

let select = $('select-sk');
select.addEventListener('selection-changed', e => {
  $('event').textContent = JSON.stringify(e.detail, null, '  ');
});

$('select-add').addEventListener('click',
  // Test MutationObserver by adding a selected element to the end.
  e => {
    let ele = document.createElement('div');
    ele.textContent = Math.random();
    ele.setAttribute('selected', '');
    select.selection = -1;
    select.appendChild(ele);
  });

