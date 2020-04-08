/**
 * @module module/multi-zoom-sk
 * @description <h2><code>multi-zoom-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from '../../../common-sk/modules/dom';
import { colorHex, digestDiffImagePath, digestImagePath } from '../common';

import 'elements-sk/checkbox-sk';

const template = (ele) => html`
<div class=container>
  <div class=preview_and_zoomed>${leftColumn(ele)}</div>
  <div class=stats_and_nav>${rightColumn(ele)}</div>
</div>`;

const leftColumn = (ele) => html`
<div class=previews_and_toggles>
  ${previewAndToggle(digestImagePath(ele._leftDigest), ele._leftLabel, 'left', ele)}
  ${previewAndToggle(digestDiffImagePath(ele._leftDigest, ele._rightDigest), 'Diff', 'diff', ele)}
  ${previewAndToggle(digestImagePath(ele._rightDigest), ele._rightLabel, 'right', ele)}
</div>
<div class=zoomed_view>
  <canvas class=zoomed width=500 height=500></canvas>
</div>
<canvas class=scratch></canvas>
`;

const previewAndToggle = (src, label, id, ele) => html`
<figure class=preview_toggle>
  <img id="img-${id}" class=thumbnail src=${src} alt=${id} @load=${() => ele._loadedImage(id)}>
  <canvas id="canvas-${id}" width=${previewCanvasSize} height=${previewCanvasSize}
      @click=${(e) => ele._snapToClick(e, id)}></canvas>
  <figcaption>${label}</figcaption>
  <checkbox-sk label="show"></checkbox-sk>
</figure>`;

const rightColumn = (ele) => html`
stats
Coord: ${ele._x}, ${ele._y}
<br>
Left: ${ele._currentColor('left')}
<br>
Diff : ${ele._currentDiff()}
<br>
Right: ${ele._currentColor('right')}
<br>
navigation
`;

const previewCanvasSize = 128;
const imageIDs = ['left', 'diff', 'right'];

const getRGBA = (imgData, X, Y) => {
  const offset = (Y * imgData.width + X) * 4;
  return [
    imgData.data[offset],
    imgData.data[offset + 1],
    imgData.data[offset + 2],
    imgData.data[offset + 3],
  ];
};

// compute how much we scaled down, if at all. Either we had to scale down because the width
// was too big, the height was too big, or no scaling was done.
const scaleOf = (originalWidth, originalHeight) => Math.min(previewCanvasSize / originalWidth,
  previewCanvasSize / originalHeight, 1);

define('multi-zoom-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._x = 0;
    this._y = 0;
    this._leftDigest = '';
    this._rightDigest = '';
    this._leftLabel = '';
    this._rightLabel = '';
    this._loadedImages = {
      left: false,
      diff: false,
      right: false,
    };
    this._loadedImageData = {};
    this._zoomLevel = 8;

    this._keyEventHandler = (e) => this._keyPressed(e);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    document.addEventListener('keyup', this._keyEventHandler);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('keyup', this._keyEventHandler);
  }

  /**
   * @prop details {object} an object with strings leftDigest, leftLabel, rightDigest and rightLabel
   */
  set details(obj) {
    this._leftDigest = obj.leftDigest || '';
    this._rightDigest = obj.rightDigest || '';
    this._leftLabel = obj.leftLabel || '';
    this._rightLabel = obj.rightLabel || '';
    this._render();
  }

  _currentColor(imgID) {
    const imgData = this._loadedImageData[imgID];
    if (!imgData) {
      return '';
    }
    const [r, g, b, a] = getRGBA(imgData, this._x, this._y);
    return `rgba(${r}, ${g}, ${b}, ${a}) ${colorHex(r, g, b, a)}`;
  }

  _currentDiff() {
    const leftData = this._loadedImageData.left;
    if (!leftData) {
      return '';
    }
    const rightData = this._loadedImageData.right;
    if (!rightData) {
      return '';
    }
    const [leftR, leftG, leftB, leftA] = getRGBA(leftData, this._x, this._y);
    const [rightR, rightG, rightB, rightA] = getRGBA(rightData, this._x, this._y);
    return `rgba(${Math.abs(leftR - rightR)}, ${Math.abs(leftG - rightG)}, `
      + `${Math.abs(leftB - rightB)}, ${Math.abs(leftA - rightA)})`;
  }

  _loadedImage(imgID) {
    this._loadedImages[imgID] = true;
    const img = $$(`#img-${imgID}`, this);
    const scratchCanvas = $$('.scratch', this);
    scratchCanvas.width = img.naturalWidth;
    scratchCanvas.height = img.naturalHeight;

    const ctx = scratchCanvas.getContext('2d');
    ctx.clearRect(0, 0, img.naturalWidth, img.naturalHeight);
    ctx.drawImage(img, 0, 0);
    this._loadedImageData[imgID] = ctx.getImageData(0, 0, img.naturalWidth, img.naturalHeight);

    this._render();
  }

  _keyPressed(e) {
    // Advice taken from https://medium.com/@uistephen/keyboardevent-key-for-cross-browser-key-press-check-61dbad0a067a
    console.log('key pressed', e);

    const key = e.key || e.keyCode;

    switch (key) {
      case 'z': case 90:
        this._zoomLevel *= 2;
        this._render();
        break;
      case 'a': case 65:
        this._zoomLevel = Math.max(1, this._zoomLevel / 2);
        this._render();
        break;
      case 'j': case 74:
        this._y += 1;
        this._render();
        break;
      case 'k': case 75:
        this._y = Math.max(0, this._y - 1);
        this._render();
        break;
      case 'l': case 76:
        this._x += 1;
        this._render();
        break;
      case 'h': case 72:
        this._x = Math.max(0, this._x - 1);
        this._render();
        break;
        // TODO(kjlubick) u, y, m
      default:
        return;
    }
    e.stopPropagation();
  }

  _snapToClick(e, imgID) {
    const imgData = this._loadedImageData[imgID];
    if (!imgData) {
      return;
    }
    const scale = scaleOf(imgData.width, imgData.height);
    console.log('click', e);
    let x = Math.round(e.offsetX / scale);
    let y = Math.round(e.offsetY / scale);
    if (x > imgData.width) {
      x = imgData.width - 1;
    }
    if (y > imgData.height) {
      y = imgData.height -1;
    }
    this._x = x;
    this._y = y;
    this._render();
  }

  _render() {
    super._render();
    for (const imgID of imageIDs) {
      if (!this._loadedImages[imgID]) {
        continue;
      }
      const canvas = $$(`#canvas-${imgID}`, this);
      const img = $$(`#img-${imgID}`, this);
      const ctx = canvas.getContext('2d');
      const scale = scaleOf(img.naturalWidth, img.naturalHeight);
      ctx.clearRect(0, 0, previewCanvasSize, previewCanvasSize);
      ctx.strokeStyle = 'red';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(this._x * scale, 0);
      ctx.lineTo(this._x * scale, previewCanvasSize);
      ctx.moveTo(0, this._y * scale);
      ctx.lineTo(previewCanvasSize, this._y * scale);
      ctx.stroke();
    }

    if (this._loadedImages.left) {
      const canvas = $$('canvas.zoomed', this);
      const img = $$('#img-left', this);
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#CCC'; // background that is not our image
      ctx.fillRect(0, 0, 500, 500);
      ctx.imageSmoothingEnabled = false;
      const x = 250 - (this._x * this._zoomLevel);
      const y = 250 - (this._y * this._zoomLevel);
      const w = img.naturalWidth * this._zoomLevel;
      const h = img.naturalHeight * this._zoomLevel;
      // Draw a white backdrop for our image, in case there are transparent pixels
      ctx.fillStyle = '#FFF';
      ctx.fillRect(x, y, w, h);
      ctx.drawImage(img, x, y, w, h);

      ctx.strokeStyle = 'black';
      ctx.lineWidth = 1;
      ctx.strokeRect(250, 250, this._zoomLevel, this._zoomLevel);
    }
  }
});
