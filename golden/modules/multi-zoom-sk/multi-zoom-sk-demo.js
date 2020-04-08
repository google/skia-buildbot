import './index';

import dialogPolyfill from 'dialog-polyfill';

import { $$ } from '../../../common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';

setImageEndpointsForDemos();
// TODO(kjlubick) detect if puppeteer and turn of the auto rotation.
let ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftDigest: '99c58c7002073346ff55f446d47d6311',
  rightDigest: '6246b773851984c726cb2e1cb13510c2',
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
$$('#normal').appendChild(ele);


ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftDigest: 'ec3b8f27397d99581e06eaa46d6d5837',
  rightDigest: '6246b773851984c726cb2e1cb13510c2',
  leftLabel: 'ec3b8f27397...',
  rightLabel: '6246b773851...',
};
$$('#mismatch').appendChild(ele);

dialogPolyfill.registerDialog($$('#the_dialog'));

$$('#in_dialog').details = {
  leftDigest: '99c58c7002073346ff55f446d47d6311',
  rightDigest: '6246b773851984c726cb2e1cb13510c2',
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};

$$('#dialog_btn').addEventListener('click', () => {
  $$('#the_dialog').showModal();
});

$$('#close_btn').addEventListener('click', () => {
  $$('#the_dialog').close();
});
