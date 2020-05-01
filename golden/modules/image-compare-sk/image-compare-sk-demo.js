import './index';
import { $$ } from 'common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';

const aDigest = '6246b773851984c726cb2e1cb13510c2';
const bDigest = '99c58c7002073346ff55f446d47d6311';

setImageEndpointsForDemos();
let ele = document.createElement('image-compare-sk');
ele.left = {
  digest: aDigest,
  title: 'a digest title',
  detail: 'example.com#aDigest',
};
ele.right = {
  digest: bDigest,
  title: 'the other image',
  detail: 'example.com#bDigest',
};
$$('#normal').appendChild(ele);

ele = document.createElement('image-compare-sk');
ele.left = {
  digest: aDigest,
  title: 'a digest title',
  detail: 'example.com#aDigest',
};
$$('#no_right').appendChild(ele);

document.addEventListener('zoom-clicked', (e) => {
  $$('#event').textContent = `zoom-clicked: ${JSON.stringify(e.detail)}`;
});
