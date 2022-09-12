import './index';

import { $$ } from 'common-sk/modules/dom';
import { digestDiffImagePath, digestImagePath } from '../common';
import { diff16x16, left16x16, right16x16 } from './test_data';
import { isPuppeteerTest } from '../demo_util';
import { MultiZoomSk } from './multi-zoom-sk';

const isPuppeteer = isPuppeteerTest();

let ele = new MultiZoomSk();
ele.cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc: digestDiffImagePath(
    '99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2',
  ),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
// These coordinates show an interesting difference.
ele.x = 77;
ele.y = 199;
$$<HTMLDivElement>('#normal')!.appendChild(ele);

ele = new MultiZoomSk();
ele.cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('ec3b8f27397d99581e06eaa46d6d5837'),
  diffImageSrc:
      digestDiffImagePath('ec3b8f27397d99581e06eaa46d6d5837', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: 'ec3b8f27397...',
  rightLabel: '6246b773851...',
};
$$<HTMLDivElement>('#mismatch')!.appendChild(ele);

ele = new MultiZoomSk();
ele.cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: left16x16,
  diffImageSrc: diff16x16,
  rightImageSrc: right16x16,
  leftLabel: 'left16x16',
  rightLabel: 'right16x16',
};
$$<HTMLDivElement>('#base64')!.appendChild(ele);

ele = new MultiZoomSk();
ele.cyclingView = !isPuppeteer;
ele.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc:
      digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};
ele.zoomLevel = 32;
ele.showGrid = true;
// These coordinates show an interesting difference.
ele.x = 77;
ele.y = 199;
$$<HTMLDivElement>('#zoomed_grid')!.appendChild(ele);

const nthPixel = new MultiZoomSk();
nthPixel.addEventListener('sources-loaded', () => {
  nthPixel.moveToNextLargestDiff(false);
});
nthPixel.cyclingView = !isPuppeteer;
nthPixel.details = {
  leftImageSrc: left16x16,
  diffImageSrc: diff16x16,
  rightImageSrc: right16x16,
  leftLabel: 'left16x16',
  rightLabel: 'right16x16',
};

$$<HTMLDivElement>('#base64_nthpixel')!.appendChild(nthPixel);

// This element demonstrates how to use multi-zoom-sk in a dialog. It is not meant for use on
// puppeteer.
$$<MultiZoomSk>('#in_dialog')!.details = {
  leftImageSrc: digestImagePath('99c58c7002073346ff55f446d47d6311'),
  diffImageSrc:
      digestDiffImagePath('99c58c7002073346ff55f446d47d6311', '6246b773851984c726cb2e1cb13510c2'),
  rightImageSrc: digestImagePath('6246b773851984c726cb2e1cb13510c2'),
  leftLabel: '99c58c700207...',
  rightLabel: 'Closest Positive',
};

$$<HTMLButtonElement>('#dialog_btn')!.addEventListener('click', () => {
  $$<HTMLDialogElement>('#the_dialog')!.showModal();
});

$$<HTMLButtonElement>('#close_btn')!.addEventListener('click', () => {
  $$<HTMLDialogElement>('#the_dialog')!.close();
});
