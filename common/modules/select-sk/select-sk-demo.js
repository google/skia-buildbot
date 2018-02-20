import 'skia-elements/buttons'

import { $$ } from 'common/dom'

import './index.js'

let select = $$('#select-sk');
select.addEventListener('selection-changed', e => {
  $$('#event').textContent = JSON.stringify(e.detail, null, '  ');
});

// Test MutationObserver by adding a selected element to the end.
$$('#select-add').addEventListener('click', e => {
  let ele = document.createElement('div');
  ele.textContent = Math.random();
  ele.setAttribute('selected', '');
  select.selection = -1;
  select.appendChild(ele);
});
