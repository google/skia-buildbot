import './index';

import dialogPolyfill from 'dialog-polyfill';

import { $$ } from '../../../common-sk/modules/dom';
import { digestDiffImagePath, digestImagePath, setImageEndpointsForDemos } from '../common';
import { diff16x16, left16x16, right16x16 } from './test_data';


setImageEndpointsForDemos();
// TODO(kjlubick) detect if puppeteer and turn of the auto rotation.
let ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc: digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
$$('#normal').appendChild(ele);


ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftImageSrc: digestImagePath('ec3b8f27397d99581e06eaa46d6d5837'),
  diffImageSrc: digestDiffImagePath('ec3b8f27397d99581e06eaa46d6d5837', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: 'ec3b8f27397...',
  rightLabel: '6246b773851...',
};
$$('#mismatch').appendChild(ele);

ele = document.createElement('multi-zoom-sk');
ele.details = {
  leftImageSrc: left16x16,
  diffImageSrc: diff16x16,
  rightImageSrc: right16x16,
  leftLabel: 'left16x16',
  rightLabel: 'right16x16',
};
$$('#mismatch').appendChild(ele);

dialogPolyfill.registerDialog($$('#the_dialog'));

$$('#in_dialog').details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc: digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};

$$('#dialog_btn').addEventListener('click', () => {
  $$('#the_dialog').showModal();
});

$$('#close_btn').addEventListener('click', () => {
  $$('#the_dialog').close();
});
