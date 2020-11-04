/**
 * @module modules/debug-view-sk
 * @description Container and manager of the wasm-linked main canvas for the debugger.
 *   Contains several CSS resizing buttons that do not alter the surface size.
 *
 * @evt move-zoom-cursor: informs zoom-sk that the user has moved the cursor
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Point, ZoomSkPointEventDetail } from '../zoom-sk/zoom-sk';
import { DebuggerPageSkLightDarkEventDetail } from '../debugger-page-sk/debugger-page-sk';

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
    <div id="backdrop" class="${ele._backdropStyle} grid">
      ${ ele._renderCanvas
      ? html`<canvas id="main-canvas" class=${ele._fitStyle}
              width=${ele._width} height=${ele._height}></canvas>`
      : '' }
      <canvas id="crosshair-canvas" class=${ele._fitStyle}
              width=${ele._width} height=${ele._height}
              @click=${ele._canvasClicked}
              @mousemove=${ele._canvasMouseMove}></canvas>
    </div>`;

  // the native width and height of the main canvas, before css is applied
  private _width: number = 400;
  private _height: number = 400;
  // the size of the canvas in pixels after css is applied
  // we need to know this because it's the coordinate space of mouse events.
  private _visibleWidth = 400;
  private _visibleHeight = 400;
  // the css class used to size the canvas.
  private _fitStyle: FitStyle = 'fit';
  private _backdropStyle = 'light-checkerboard';
  private _crossHairActive = false;
  private _renderCanvas = true;

  constructor() {
    super(DebugViewSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    // when the user moves the cursor using the zoom window
    document.addEventListener('zoom-point', (e) => {
      const cursor = (e as CustomEvent<ZoomSkPointEventDetail>).detail.position;
      if (this._crossHairActive) {
        this._drawCrossHairAt(cursor);
      }
    });

    document.addEventListener('light-dark', (e) => {
      this._backdropStyle = (e as CustomEvent<DebuggerPageSkLightDarkEventDetail>).detail.mode;
      this._render();
    });
  }

  // Pass one of the CSS classes for sizing the debug view canvas.
  // It doesn't change the pixel size of the SkSurface, use resize for that.
  set fitStyle(fs: FitStyle) {
    this._fitStyle = fs;
    this._render();
    this._visibleSize();
  }

  get canvas(): HTMLCanvasElement {
    this._render();
    return this.querySelector<HTMLCanvasElement>('#main-canvas')!
  }

  // Replace the main canvas element, changing its native size
  resize(width = 400, height = 400): HTMLCanvasElement {
    this._width = width;
    this._height = height;
    this._renderCanvas = false;
    this._render(); // delete it to clear it's rendering context.
    this._renderCanvas = true;
    this._render(); // template makes a fresh one.
    return this.querySelector('canvas')!;
  }

  private _visibleSize() {
    const element = this.querySelector<HTMLCanvasElement>('#main-canvas')!;
    var strW = window.getComputedStyle(element, null).width;
    var strH = window.getComputedStyle(element, null).height;
    // Trim 'px' off the end of the style string and convert to a number.
    this._visibleWidth = parseFloat(strW.substring(0, strW.length-2));
    this._visibleHeight = parseFloat(strH.substring(0, strH.length-2));
  }

  private _mouseOffsetToCanvasPoint(e: MouseEvent): Point {
    return [
      Math.round(e.offsetX / this._visibleWidth * this._width),
      Math.round(e.offsetY / this._visibleHeight * this._height),
    ];
  }

  private _sendCursorMove(p: Point) {
    this.dispatchEvent(
      new CustomEvent<ZoomSkPointEventDetail>(
        'move-zoom-cursor', {
          detail: {position: p},
          bubbles: true,
        }));
  }

  private _drawCrossHairAt(p: Point) {
    const chCanvas = this.querySelector<HTMLCanvasElement>('#crosshair-canvas')!;
    const chx = chCanvas.getContext('2d')!;
    chx.clearRect(0, 0, chCanvas.width, chCanvas.height);

    chx.lineWidth =  1;
    chx.strokeStyle = '#F00';
    chx.beginPath();
    chx.moveTo(0, p[1]-0.5);
    chx.lineTo(chCanvas.width+1, p[1]-0.5);
    chx.moveTo(p[0]-0.5, 0);
    chx.lineTo(p[0]-0.5, chCanvas.height+1);
    chx.stroke();
  }

  private _canvasClicked(e: MouseEvent) {
    if (e.offsetX < 0) { return; } // border
    if (this._crossHairActive) {
      this._crossHairActive = false;
      this._drawCrossHairAt([-5, -5]);
    } else {
      this._crossHairActive = true;
      const coords = this._mouseOffsetToCanvasPoint(e);
      this._drawCrossHairAt(coords);
      this._sendCursorMove(this._mouseOffsetToCanvasPoint(e));
    }
  }

  private _canvasMouseMove(e: MouseEvent) {
    if (e.offsetX < 0) { return; } // border
    if (this._crossHairActive) { return; }
    this._sendCursorMove(this._mouseOffsetToCanvasPoint(e));
  }
};

define('debug-view-sk', DebugViewSk);
