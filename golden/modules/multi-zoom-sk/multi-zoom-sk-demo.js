import './index';

import dialogPolyfill from 'dialog-polyfill';

import { $$ } from '../../../common-sk/modules/dom';
import { digestDiffImagePath, digestImagePath, setImageEndpointsForDemos } from '../common';
import { diff16x16, left16x16, right16x16 } from './test_data';
import { isPuppeteerTest } from '../demo_util';

setImageEndpointsForDemos();

const isPuppeteer = isPuppeteerTest();

let ele = document.createElement('multi-zoom-sk');
ele._cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc: digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
// These coordinates show an interesting difference.
ele._x = 77;
ele._y = 199;
$$('#normal').appendChild(ele);


ele = document.createElement('multi-zoom-sk');
ele._cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('ec3b8f27397d99581e06eaa46d6d5837'),
  diffImageSrc: digestDiffImagePath('ec3b8f27397d99581e06eaa46d6d5837', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: 'ec3b8f27397...',
  rightLabel: '6246b773851...',
};
$$('#mismatch').appendChild(ele);

ele = document.createElement('multi-zoom-sk');
ele._cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: left16x16,
  diffImageSrc: diff16x16,
  rightImageSrc: right16x16,
  leftLabel: 'left16x16',
  rightLabel: 'right16x16',
};
$$('#base64').appendChild(ele);

ele = document.createElement('multi-zoom-sk');
ele._cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc: digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
ele._zoomLevel = 32;
ele._showGrid = true;
// These coordinates show an interesting difference.
ele._x = 77;
ele._y = 199;
$$('#zoomed_grid').appendChild(ele);

const nthPixel = document.createElement('multi-zoom-sk');
nthPixel.addEventListener('sources-loaded', () => {
  nthPixel._moveToNextLargestDiff(false);
});
nthPixel._cyclingView = !isPuppeteer;
nthPixel.details = {
  leftImageSrc: left16x16,
  diffImageSrc: diff16x16,
  rightImageSrc: right16x16,
  leftLabel: 'left16x16',
  rightLabel: 'right16x16',
};

$$('#base64_nthpixel').appendChild(nthPixel);

// This element demonstrates how to use multi-zoom-sk in a dialog. It is not meant for use on
// puppeteer.
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
