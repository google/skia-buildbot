import './index';

import { $$ } from 'common-sk/modules/dom';

$$('#first').key = 'age';
$$('#second').key = 'height';

$$('#first').currentKey = 'age';
$$('#second').currentKey = 'age';

$$('#first').direction = 'desc';

$$('#two_sorts').addEventListener('sort-change', (e) => {
  console.log('click', e.detail);
  $$('#first').currentKey = e.detail.key;
  $$('#second').currentKey = e.detail.key;

  $$('#log').textContent = `Now sorting by ${e.detail.key} in the direction ${e.detail.direction}`;
});
