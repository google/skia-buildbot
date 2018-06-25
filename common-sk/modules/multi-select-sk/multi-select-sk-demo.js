import 'elements-sk/buttons'

import { $$ } from '../dom'

import './index.js'

let select = $$('#multi-select-sk');
select.addEventListener('selection-changed', e => {
  $$('#event').textContent = JSON.stringify(e.detail, null, '  ');
});

// Test MutationObserver by adding a selected element to the end.
$$('#select-add').addEventListener('click', e => {
  let ele = document.createElement('div');
  ele.textContent = 'should be selected' + Math.random().toFixed(3);
  ele.setAttribute('selected', '');
  select.appendChild(ele);
});

$$('#other-add').addEventListener('click', e => {
  let ele = document.createElement('div');
  ele.textContent = 'should not be selected' + Math.random().toFixed(3);
  select.appendChild(ele);
});
