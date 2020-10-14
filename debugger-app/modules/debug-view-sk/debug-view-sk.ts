/**
 * @module modules/debug-view-sk
 * @description Container and manager of the wasm-linked main canvas for the debugger.
 *   Contains several CSS resizing buttons that do not alter the surface size.
 *   TODO(nifong): Render a crosshair for selecting a pixel of the canvas
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export type FitStyle = 'natural' | 'fit' | 'right' | 'bottom';

export class DebugViewSk extends ElementSk {
  private static template = (ele: DebugViewSk) =>
    html`
    <div class="horizontal-flex">
      <button title="Original size." @click=${() => ele.fitStyle = 'natural'}>
        <img src="https://debugger-assets.skia.org/res/img/image.png" />
      </button>
      <button title="Fit in page." @click=${() => ele.fitStyle = 'fit'}>
        <img src="https://debugger-assets.skia.org/res/img/both.png" />
      </button>
      <button title="Fit to width." @click=${() => ele.fitStyle = 'right'}>
        <img src="https://debugger-assets.skia.org/res/img/right.png" />
      </button>
      <button title="Fit to height." @click=${() => ele.fitStyle = 'bottom'}>
        <img src="https://debugger-assets.skia.org/res/img/bottom.png" />
      </button>
    </div>
    <div class="light-checkerboard">
      <canvas class=${ele._fitStyle} width=${ele._width} height=${ele._height}></canvas>
    </div>`;

  // the native width and height of the main canvas, before css is applied
  private _width: number = 400;
  private _height: number = 400;
  private _fitStyle: FitStyle = 'right';

  constructor() {
    super(DebugViewSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // Pass one of the CSS classes for sizing the debug view canvas.
  // It doesn't change the pixel size of the SkSurface, use resize for that.
  set fitStyle(fs: FitStyle) {
    this._fitStyle = fs;
    this._render();
  }

  // TODO(nifong): figure out if template rendering is sufficient to clear the
  // WebGlContext. You need to test it with the cpu/gpu switch.
  // It may be necessary to swtich back to this commented out method.

  // // Replace the main canvas element, changing its native size
  // replaceMainCanvas(width?: number, height?: number) : HTMLCanvasElement {
  //   const dvcanvas = <HTMLCanvasElement> document.getElementById('maincanvas');
  //   width = width || 400;
  //   height = height || 400;
  //   // Discard canvas when switching between cpu/gpu backend because its bound to a Web GL context.
  //   const newCanvas = <HTMLCanvasElement> dvcanvas.cloneNode(true);
  //   dvcanvas.replaceWith(newCanvas);
  //   newCanvas.width = width;
  //   newCanvas.height = height;
  //   return newCanvas;
  // }

  // Replace the main canvas element, changing its native size
  resize(width: number, height: number): HTMLCanvasElement {
    this._width = width;
    this._height = height;
    this._render();
    return this.querySelector('canvas')!;
  }
};

define('debug-view-sk', DebugViewSk);
