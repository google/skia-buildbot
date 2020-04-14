/**
 * @module module/multi-zoom-sk
 * @description <h2><code>multi-zoom-sk</code></h2>
 *
 * The multi-zoom-sk element shows a zoomed-in comparison between two images (with a rendered diff).
 * It supports many keybindings for navigation of the image, as well as clicking on the image
 * thumbnails to navigate around.
 *
 * @event sources-loaded when the left, the right, and the diff image sources have been loaded.
 *
 * It should typically be wrapped in a dialog tag.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from '../../../common-sk/modules/dom';

import 'elements-sk/checkbox-sk';

const previewCanvasSize = 128;
const zoomedCanvasSize = 500;
const leftImageIdx = 0;
const diffImageIdx = 1;
const rightImageIdx = 2;

const template = (ele) => html`
<div class=container>
  <div class=preview_and_zoomed>${previewsAndZoomCanvas(ele)}</div>
  <div class=stats_and_nav>${statsAndNavigation(ele)}</div>
</div>`;

const previewsAndZoomCanvas = (ele) => html`
<div class=previews_and_toggles>
  ${thumbnailAndToggle(ele, leftImageIdx)}
  ${thumbnailAndToggle(ele, diffImageIdx)}
  ${thumbnailAndToggle(ele, rightImageIdx)}
</div>
<div class=zoomed_view>
  <canvas class=zoomed width=${zoomedCanvasSize} height=${zoomedCanvasSize}></canvas>
</div>
<!-- This scratch canvas is not displayed, but is used to get the pixel data from the
     loaded images-->
<canvas class=scratch></canvas>
`;

// thumbnailAndToggle dynamically creates an img, a canvas and a checkbox-sk with classes "idx_N"
// We chose classes instead of ids because on the off-chance there are multiple of these elements
// on the page, it is best to not have duplicate ids. The image and canvas will be used to have the
// thumbnail and the crosshair over it. The crosshair shows the user where in the image they are
// zoomed into. The checkbox is used to select which images should be toggled through in the zoomed
// element.
const thumbnailAndToggle = (ele, idx) => {
  const label = ele._labels[idx];

  return html`
<figure class=preview>
  <img class="thumbnail idx_${idx}" src=${ele._srcs[idx]} alt=${label}
      @load=${() => ele._imageLoaded(idx)}>
  <canvas class="crosshair idx_${idx}" width=${previewCanvasSize} height=${previewCanvasSize}
      @click=${(e) => ele._previewCanvasClicked(e, idx)}></canvas>
  <figcaption>
    <checkbox-sk label=${label} class="displayed for_spacing"></checkbox-sk>
    <checkbox-sk label=${label} class="idx_${idx} ${idx === ele._zoomedIndex ? 'displayed' : ''}"
        ?checked=${ele._cycleThrough[idx]}  @click=${(e) => ele._cycleBoxClicked(e, idx)}>
    </checkbox-sk>
  </figcaption>
</figure>`;
};

const statsAndNavigation = (ele) => html`
<table class=stats>
  <tr>
    <td class=label>Coordinate</td>
    <td class="coord value">(${ele._x}, ${ele._y})</td>
  </tr>
  <tr>
    <td class=label>Left Pixel</td>
    <td class="left value">${ele._currentColor(leftImageIdx)}</td>
  </tr>
  <tr>
    <td class=label>Diff</td>
    <td class="diff value">${ele._currentDiff()}</td>
  </tr>
  <tr>
    <td class=label>Right Pixel</td>
    <td class="right value">${ele._currentColor(rightImageIdx)}</td>
  </tr>
</table>
<!-- TODO(kjlubick) Here would be a good place for reading any trace comments and putting pixel
     specific ones here.-->
${nthPixelDiff(ele)}
<table class=navigation>
  <tr><th colspan=2>Navigation</td></tr>
  <tr><td class=label>H</td><td>Left</td></tr>
  <tr><td class=label>J</td><td>Down</td></tr>
  <tr><td class=label>K</td><td>Up</td></tr>
  <tr><td class=label>L</td><td>Right</td></tr>
  <tr><td class=label>A</td><td>Zoom Out</td></tr>
  <tr><td class=label>Z</td><td>Zoom In</td></tr>
  <tr><td class=label>U</td><td>Jump To Next Largest Diff</td></tr>
  <tr><td class=label>Y</td><td>Jump To Prev. Largest Diff</td></tr>
  <tr><td class=label>M</td><td>Manual Toggle</td></tr>
  <tr><td class=label>G</td><td>Hide/Show Grid</td></tr>
</table>
`;

// If the pixel diffs between the two images have been calculated and sorted, look up to see if
// the given pixel is in the list. If so, display where the diff is in the ordering.
const nthPixelDiff = (ele) => {
  if (!ele._cachedDiffs || !ele._cachedDiffs.length) {
    return '';
  }
  const endings = ['st', 'nd', 'rd']; // for 1st, 2nd, 3rd
  const total = ele._cachedDiffs.length;
  for (let i = 0; i < total; i++) {
    const d = ele._cachedDiffs[i];
    if (d.x === ele._x && d.y === ele._y) {
      // Update our current diff index so that if the user navigates (using
      // J/H/K/L) from the 3rd to the 12th biggest pixel and hits U, they
      // go to the 13th biggest diff, not the 4th.
      ele._cachedDiffIdx = i;
      const e = endings[i] || 'th';
      return html`<div class=nth_diff>${i + 1}${e} biggest pixel diff (out of ${total})</div>`;
    }
  }
  return html`<div class=nth_diff>No difference on this pixel</div>`;
};

// min and max are inclusive.
const clamp = (n, min, max) => {
  if (n < min) {
    return min;
  }
  if (n > max) {
    return max;
  }
  return n;
};

const getRGBA = (imgData, X, Y) => {
  const offset = (Y * imgData.width + X) * 4;
  return [
    imgData.data[offset],
    imgData.data[offset + 1],
    imgData.data[offset + 2],
    imgData.data[offset + 3],
  ];
};

// colorHex returns a hex representation of a given color pixel as a string.
function colorHex(r, g, b, a) {
  const toHex = (i) => i.toString(16).toUpperCase().padStart(2, '0');

  return `#${toHex(r)}${toHex(g)}${toHex(b)}${toHex(a)}`;
}

// colorDist returns the distance of a color from (0, 0, 0, 0) using a
// crude square distance per channel.
const colorDist = (r, g, b, a) => r * r + g * g + b * b + a * a;

// Compute how much we scaled down, if at all. Either we had to scale down because the width
// was too big, the height was too big, or no scaling was done.
const scaleOf = (originalWidth, originalHeight) => Math.min(previewCanvasSize / originalWidth,
  previewCanvasSize / originalHeight, 1);

define('multi-zoom-sk', class extends ElementSk {
  constructor() {
    super(template);

    // _x and _y are in the native image coordinates; that is, they are not scaled.
    this._x = 0;
    this._y = 0;
    this._srcs = ['', '', ''];
    this._labels = ['', 'Diff', ''];

    // We save the image data from all 3 images after it loads here - this lets us access the
    // pixel data quickly.
    this._loadedImageData = [null, null, null];
    // How many times are we zoomed in. We default to 8x.
    this._zoomLevel = 8;
    // The index of the image we should be zoomed in. -1 is a sentinel value for none.
    this._zoomedIndex = 0;
    this._showGrid = false;
    // If we are cycling through a subset of the images.
    this._cyclingView = true;
    // Default to rotating through left and right image (i.e. index 0 and index 2).
    this._cycleThrough = [true, false, true];
    // Used by the u/y key presses to go through pixels that are different between the right and
    // left image.
    this._cachedDiffs = [];
    this._cachedDiffIdx = -1;

    this._keyEventHandler = (e) => this._keyPressed(e);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // This assumes that there is only one multi-zoom-sk rendered on the page at a time (if there
    // are multiple, they may all respond to keypresses at once).
    document.addEventListener('keydown', this._keyEventHandler);
    // Every 1 second (1000 ms), go to the next image that the user has checked the box for, if
    // there is one.
    const maybeCycleZoomedImage = () => {
      if (!this._cyclingView) {
        return; // The user must have manually started to cycle images, thus we stop.
      }
      // Unless our element has been removed from the DOM, reschedule another check.
      if (this._connected) {
        setTimeout(maybeCycleZoomedImage, 1000);
      } else {
        return;
      }
      this._nextZoomedImage();
    };

    setTimeout(maybeCycleZoomedImage, 1000);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this._keyEventHandler);
    // Free up heavy resources. This may not be necessary, but it makes sure we aren't erroneously
    // holding onto them.
    this._cachedDiffs = [];
    this._loadedImageData = [null, null, null];
  }

  /**
   * @prop details {object} an object with strings leftImageSrc, rightImageSrc, diffImageSrc,
   *    leftLabel, and rightLabel. These control what to draw and compare.
   */
  set details(obj) {
    this._srcs[leftImageIdx] = obj.leftImageSrc || '';
    this._srcs[diffImageIdx] = obj.diffImageSrc || '';
    this._srcs[rightImageIdx] = obj.rightImageSrc || '';
    this._labels[leftImageIdx] = obj.leftLabel || '';
    this._labels[rightImageIdx] = obj.rightLabel || '';
    // Clear the cache of differences. We'll need to recompute them when the images load.
    this._cachedDiffs = [];
    this._cachedDiffIdx = -1;
    this._render();
  }

  _buildPixelDiffCache() {
    const leftData = this._loadedImageData[leftImageIdx];
    const rightData = this._loadedImageData[rightImageIdx];
    // find all the diffs and sort them biggest diff to smallest diff.
    const width = Math.min(leftData.width, rightData.width);
    const height = Math.min(leftData.height, rightData.height);

    this._cachedDiffs = [];
    for (let x = 0; x < width; x++) {
      for (let y = 0; y < height; y++) {
        const [leftR, leftG, leftB, leftA] = getRGBA(leftData, x, y);
        const [rightR, rightG, rightB, rightA] = getRGBA(rightData, x, y);
        const dist = colorDist(leftR - rightR, leftG - rightG, leftB - rightB,
          leftA - rightA);
        if (!dist) {
          // No difference in pixels - no need to add
          // it to our list of "different pixels"
          continue;
        }
        this._cachedDiffs.push({
          x: x,
          y: y,
          diff: dist,
        });
      }
    }
    this._cachedDiffs.sort((a, b) => {
      // First sort diffs high to low, so biggest ones are first
      const d = b.diff - a.diff;
      if (d) {
        return d;
      }
      // prioritize up and to the left for tie breaks.
      if (b.x !== a.x) {
        return a.x - b.x;
      }
      return a.y - b.y;
    });
  }

  _currentColor(imgIdx) {
    const imgData = this._loadedImageData[imgIdx];
    if (!imgData) {
      return '';
    }
    if (this._x >= imgData.width || this._y >= imgData.height) {
      return 'out of bounds';
    }
    const [r, g, b, a] = getRGBA(imgData, this._x, this._y);
    return `rgba(${r}, ${g}, ${b}, ${a}) ${colorHex(r, g, b, a)}`;
  }

  _currentDiff() {
    const leftData = this._loadedImageData[leftImageIdx];
    const rightData = this._loadedImageData[rightImageIdx ];
    if (!leftData || !rightData) {
      return '';
    }
    if (this._x >= leftData.width || this._y >= leftData.height
      || this._x >= rightData.width || this._y >= rightData.height) {
      return 'n/a';
    }
    const [leftR, leftG, leftB, leftA] = getRGBA(leftData, this._x, this._y);
    const [rightR, rightG, rightB, rightA] = getRGBA(rightData, this._x, this._y);
    return `rgba(${Math.abs(leftR - rightR)}, ${Math.abs(leftG - rightG)}, `
      + `${Math.abs(leftB - rightB)}, ${Math.abs(leftA - rightA)})`;
  }

  _cycleBoxClicked(e, imgIdx) {
    e.stopPropagation();
    this._cycleThrough[imgIdx] = !this._cycleThrough[imgIdx];
    // Nothing was selected previously, so snap to the new selection.
    if (this._zoomedIndex < 0 && this._cycleThrough[imgIdx]) {
      this._zoomedIndex = imgIdx;
    }
    this._render();
  }

  _drawCrosshairsOnPreview(imgIdx) {
    const imgData = this._loadedImageData[imgIdx];
    if (!imgData) {
      return;
    }
    const canvas = this._getCrosshairCanvas(imgIdx);
    const ctx = canvas.getContext('2d');
    const scale = scaleOf(imgData.width, imgData.height);
    ctx.clearRect(0, 0, previewCanvasSize, previewCanvasSize);
    ctx.strokeStyle = 'red';
    ctx.lineWidth = 1;
    // As specified in the docs, clearRect only works if the next path drawn starts with
    // beginPath(); Otherwise, the old path is drawn again, which means we have multiple
    // crosshairs at once.
    ctx.beginPath();
    ctx.moveTo(this._x * scale, 0);
    ctx.lineTo(this._x * scale, previewCanvasSize);
    ctx.moveTo(0, this._y * scale);
    ctx.lineTo(previewCanvasSize, this._y * scale);
    ctx.stroke();
  }

  // Draws the currently selected image on the big canvas, zoomed in according to the set level.
  _drawZoomedView(imgIdx) {
    if (this._loadedImageData[imgIdx]) {
      const canvas = $$('canvas.zoomed', this);
      const img = this._getImage(imgIdx);
      const ctx = canvas.getContext('2d');
      ctx.fillStyle = '#CCC'; // Grey background for outside the bounds of the image.
      ctx.fillRect(0, 0, zoomedCanvasSize, zoomedCanvasSize);
      ctx.imageSmoothingEnabled = false;
      // The offset from zoomedCanvasSize / 2 is to center the image.
      const x = zoomedCanvasSize / 2 - (this._x * this._zoomLevel);
      const y = zoomedCanvasSize / 2 - (this._y * this._zoomLevel);
      const w = img.naturalWidth * this._zoomLevel;
      const h = img.naturalHeight * this._zoomLevel;
      // Draw a white backdrop for our image, in case there are transparent pixels
      ctx.fillStyle = '#FFF';
      ctx.fillRect(x, y, w, h);
      // Draw the image. The canvas will clip any pixels not on the screen for us.
      ctx.drawImage(img, x, y, w, h);

      // The grid is essentially pointless when not zoomed in, so don't bother drawing it then.
      if (this._showGrid && this._zoomLevel >= 4) {
        ctx.beginPath();
        ctx.strokeStyle = '#FFF';
        ctx.lineWidth = 1;
        // This modular arithmetic lines up the grid with the central pixel. The offset by 0.5
        // makes sure we draw all within one pixel, not spread between two pixels.
        // https://stackoverflow.com/a/10003573
        let x = ((zoomedCanvasSize / 2) % this._zoomLevel) - 0.5;
        for (; x < zoomedCanvasSize; x += this._zoomLevel) {
          ctx.moveTo(x, 0);
          ctx.lineTo(x, zoomedCanvasSize);
        }
        let y = ((zoomedCanvasSize / 2) % this._zoomLevel) - 0.5;
        for (; y < zoomedCanvasSize; y += this._zoomLevel) {
          ctx.moveTo(0, y);
          ctx.lineTo(zoomedCanvasSize, y);
        }
        ctx.stroke();
      }

      // Draw the box showing the selected pixel (the center of the screen).
      ctx.strokeStyle = '#000';
      ctx.lineWidth = 1;
      // Offset by 0.5 to make sure draw a clean 1px-wide black line, not a 2px-wide grey line.
      // See above for more details.
      const corner = zoomedCanvasSize / 2 - 0.5;
      ctx.strokeRect(corner, corner, this._zoomLevel, this._zoomLevel);
    }
  }

  _getCrosshairCanvas(imgIdx) {
    return $$(`canvas.idx_${imgIdx}`, this);
  }

  _getImage(imgIdx) {
    return $$(`img.idx_${imgIdx}`, this);
  }

  _imageLoaded(imgIdx) {
    // To get the image data for the image that just loaded, we draw it into our scratch canvas and
    // then read it back.
    const img = this._getImage(imgIdx);
    const scratchCanvas = $$('canvas.scratch', this);
    scratchCanvas.width = img.naturalWidth;
    scratchCanvas.height = img.naturalHeight;

    const ctx = scratchCanvas.getContext('2d');
    ctx.clearRect(0, 0, img.naturalWidth, img.naturalHeight);
    ctx.drawImage(img, 0, 0);
    this._loadedImageData[imgIdx] = ctx.getImageData(0, 0, img.naturalWidth, img.naturalHeight);

    this._render();
    if (this._loadedImageData[leftImageIdx] && this._loadedImageData[diffImageIdx]
      && this._loadedImageData[rightImageIdx]) {
      this.dispatchEvent(new CustomEvent('sources-loaded', { bubbles: true }));
    }
  }

  _keyPressed(e) {
    // Advice taken from https://medium.com/@uistephen/keyboardevent-key-for-cross-browser-key-press-check-61dbad0a067a
    const zoomData = this._loadedImageData[this._zoomedIndex];
    const key = e.key || e.keyCode;
    switch (key) {
      case 'z': case 90: // Zoom in
        this._zoomLevel = clamp(this._zoomLevel * 2, 1, 128);
        this._render();
        break;
      case 'a': case 65: // Zoom out
        this._zoomLevel = clamp(this._zoomLevel / 2, 1, 128);
        this._render();
        break;
      case 'j': case 74: // Go down
        if (zoomData) {
          this._y = clamp(this._y + 1, 0, zoomData.height - 1);
          this._render();
        }
        break;
      case 'k': case 75: // Go up
        if (zoomData) {
          this._y = clamp(this._y - 1, 0, zoomData.height - 1);
          this._render();
        }
        break;
      case 'l': case 76: // Move right
        if (zoomData) {
          this._x = clamp(this._x + 1, 0, zoomData.width - 1);
          this._render();
        }
        break;
      case 'h': case 72: // Move left
        if (zoomData) {
          this._x = clamp(this._x - 1, 0, zoomData.width - 1);
          this._render();
        }
        break;
      case 'm': case 77: // Manually cycle to next image.
        this._cyclingView = false;
        this._nextZoomedImage();
        break;
      case 'g': case 71: // Toggle the grid.
        this._showGrid = !this._showGrid;
        this._render();
        break;
      case 'u': case 85: // move to next largest pixel diff.
        this._moveToNextLargestDiff(false);
        break;
      case 'y': case 89: // move to previous next largest pixel diff.
        this._moveToNextLargestDiff(true);
        break;
      default:
        return;
    }
    // If we captured the key event, stop it from propagating.
    e.stopPropagation();
  }

  _moveToNextLargestDiff(backwards) {
    const leftData = this._loadedImageData[leftImageIdx];
    const rightData = this._loadedImageData[rightImageIdx];
    if (!leftData || !rightData) {
      return;
    }
    if (!this._cachedDiffs || !this._cachedDiffs.length) {
      this._buildPixelDiffCache();
    }

    if (backwards) {
      this._cachedDiffIdx = clamp(this._cachedDiffIdx - 1, 0, this._cachedDiffs.length - 1);
    } else {
      this._cachedDiffIdx = clamp(this._cachedDiffIdx + 1, 0, this._cachedDiffs.length - 1);
    }

    const diff = this._cachedDiffs[this._cachedDiffIdx];
    if (!diff) {
      // Perhaps there are no diffs?
      return;
    }
    this._x = diff.x;
    this._y = diff.y;
    this._render();
  }

  _nextZoomedImage() {
    // Cycle through our 3 possible images looking for the first one that is selected to be rotated
    // through.
    for (let i = 0; i < 3; i++) {
      this._zoomedIndex = (this._zoomedIndex + 1) % 3;
      if (this._cycleThrough[this._zoomedIndex]) {
        this._render();
        return; // we've found a selected index.
      }
    }
    // None of the 3 indices are valid, set to -1 to mean "don't show anything"
    this._zoomedIndex = -1;
  }

  _previewCanvasClicked(e, imgIdx) {
    const imgData = this._loadedImageData[imgIdx];
    if (!imgData) {
      return;
    }
    const scale = scaleOf(imgData.width, imgData.height);
    // offsetX and offsetY are in scaled coordinates; by dividing by scale and then rounding them,
    // we convert them approximately onto the unscaled coordinates.
    const x = Math.round(e.offsetX / scale);
    const y = Math.round(e.offsetY / scale);

    this._x = clamp(x, 0, imgData.width - 1);
    this._y = clamp(y, 0, imgData.height - 1);
    this._render();
  }

  _render() {
    super._render();
    // HTML elements are in place, draw on our canvases now.
    for (let imgIdx = 0; imgIdx < 3; imgIdx++) {
      this._drawCrosshairsOnPreview(imgIdx);
    }
    this._drawZoomedView(this._zoomedIndex);
  }
});
