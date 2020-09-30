/**
 * @module modules/debug-view-sk
 * @description <h2><code>debug-view-sk</code></h2>
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

export class DebugViewSk extends ElementSk {
  private static template = (ele: DebugViewSk) =>
    html`
    <div class="horizontal-flex">
      <button title="Original size." @click=${() => ele._resize('natural')}>
        <img src="https://debugger-assets.skia.org/res/img/image.png" />
      </button>
      <button title="Fit in page." @click=${() => ele._resize('fit')}>
        <img src="https://debugger-assets.skia.org/res/img/both.png" />
      </button>
      <button title="Fit to width." @click=${() => ele._resize('right')}>
        <img src="https://debugger-assets.skia.org/res/img/right.png" />
      </button>
      <button title="Fit to height." @click=${() => ele._resize('bottom')}>
        <img src="https://debugger-assets.skia.org/res/img/bottom.png" />
      </button>
    </div>
    <div class="light-checkerboard">
      <canvas id="img" class="fit" width=400 height=400></canvas>
    </div>`;

  constructor() {
    super(DebugViewSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // pass one of the CSS classes for sizing the debug view canvas.
  // It doesn't change the pixel size of the SkSurface though that would be potentially useful too.
  private _resize(sizeStyle: string) {

  }
};

define('debug-view-sk', DebugViewSk);
