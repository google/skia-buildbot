/**
 * @module modules/debug-view-sk
 * @description Container and manager of the wasm-linked main canvas for the debugger.
 *   Contains several CSS resizing buttons that do not alter the surface size.
 *   TODO(nifong): Render a crosshair for selecting a pixel of the canvas
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

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
      <canvas id="maincanvas" class=${ele._fitStyle} width=${ele._width} height=${ele._width}></canvas>
    </div>`;

  // the native width and height of the main canvas, before css is applied
  private _width: number = 400;
  private _height: number = 400;
  private _fitStyle: string = 'right';

  constructor() {
    super(DebugViewSk.template);;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // Pass one of the CSS classes for sizing the debug view canvas.
  // It doesn't change the pixel size of the SkSurface, use replaceSurface for that.
  set fitStyle(fs: string) {
    // const dvcanvas = <HTMLCanvasElement> document.getElementById('maincanvas');
    // // Remove any of the 4 fit classes that may be present and apply the requested one.
    // // The debugger canvas is only meant to have one of these at a time. Each one corresponds
    // // to one of the fit buttons and sizes the canvas in a different way.
    // dvcanvas.classList.remove('natural', 'fit', 'right', 'bottom');
    // dvcanvas.classList.add(sizeStyle);
    this._fitStyle = fs;
    this._render();
  }

  // TODO(nifong): figure out if template rendering is sufficient to clear the WebGlContext
  // you need to test it with the cpu/gpu switch.

  // // Replace the main canvas element, changing it's native size
  // replaceMainCanvas(width?: number, height?: number) : HTMLCanvasElement {
  //   const dvcanvas = <HTMLCanvasElement> document.getElementById('maincanvas');
  //   width = width || 400;
  //   height = height || 400;
  //   // Discard canvas when switching between cpu/gpu backend because it's bound to a Web GL context.
  //   const newCanvas = <HTMLCanvasElement> dvcanvas.cloneNode(true);
  //   dvcanvas.replaceWith(newCanvas);
  //   newCanvas.width = width;
  //   newCanvas.height = height;
  //   return newCanvas;
  // }

  // Replace the main canvas element, changing it's native size
  resize(width: number, height: number) : HTMLCanvasElement {
    this._width = width;
    this._height = height;
    this._render();
    return <HTMLCanvasElement> document.getElementById('maincanvas');
  }
};

define('debug-view-sk', DebugViewSk);
