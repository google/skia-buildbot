/**
 * @module modules/debug-view-sk
 * @description Container and manager of the wasm-linked main canvas for the debugger.
 *   Contains several CSS resizing buttons that do not alter the surface size.
 *
 * @evt move-cursor: Emitted when the user has moved the cursor by clicking or hovering.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementDocSk } from '../element-doc-sk/element-doc-sk';
import {
  CursorEventDetail,
  ToggleBackgroundEventDetail, MoveCursorEvent,
  Point,
  RenderCursorEvent,
  ToggleBackgroundEvent,
} from '../events';

export type FitStyle = 'natural' | 'fit' | 'right' | 'bottom';

export class DebugViewSk extends ElementDocSk {
  private static template = (ele: DebugViewSk) => html`
    <div class="horizontal-flex">
      <button title="Original size." @click=${() => ele.fitStyle = 'natural'}>
        <img src="/dist/image.png" />
      </button>
      <button title="Fit in page." @click=${() => ele.fitStyle = 'fit'}>
        <img src="/dist/both.png" />
      </button>
      <button title="Fit to width." @click=${() => ele.fitStyle = 'right'}>
        <img src="/dist/right.png" />
      </button>
      <button title="Fit to height." @click=${() => ele.fitStyle = 'bottom'}>
        <img src="/dist/bottom.png" />
      </button>
    </div>
    <div id="backdrop" class="${ele._backdropStyle} grid">
      ${ele._renderCanvas
    ? html`<canvas id="main-canvas" class=${ele._fitStyle}
              width=${ele._width} height=${ele._height}></canvas>`
    : ''}
      <canvas id="crosshair-canvas" class=${ele._fitStyle}
              width=${ele._width} height=${ele._height}
              @click=${ele._canvasClicked}
              @mousemove=${ele._canvasMouseMove}></canvas>
    </div>`;

  // the native width and height of the main canvas, before css is applied
  private _width: number = 400;

  private _height: number = 400;

  // the css class used to size the canvas.
  private _fitStyle: FitStyle = 'fit';

  private _backdropStyle = 'light-checkerboard';

  private _crossHairActive = false;

  private _renderCanvas = true;

  get crosshairActive(): boolean {
    return this._crossHairActive;
  }

  constructor() {
    super(DebugViewSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.addDocumentEventListener(RenderCursorEvent, (e) => {
      const detail = (e as CustomEvent<CursorEventDetail>).detail;
      if (!this._crossHairActive || detail.onlyData) {
        return;
      }
      this._drawCrossHairAt(detail.position);
    });

    this.addDocumentEventListener(ToggleBackgroundEvent, (e) => {
      this._backdropStyle = (e as CustomEvent<ToggleBackgroundEventDetail>).detail.mode;
      this._render();
    });
  }

  // Pass one of the CSS classes for sizing the debug view canvas.
  // It doesn't change the pixel size of the SkSurface, use resize for that.
  set fitStyle(fs: FitStyle) {
    this._fitStyle = fs;
    this._render();
  }

  get canvas(): HTMLCanvasElement {
    this._render();
    return this.querySelector<HTMLCanvasElement>('#main-canvas')!;
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

  private _visibleSize(): Point {
    const element = this.querySelector<HTMLCanvasElement>('#main-canvas')!;
    const strW = window.getComputedStyle(element, null).width;
    const strH = window.getComputedStyle(element, null).height;
    // Trim 'px' off the end of the style string and convert to a number.
    const visibleWidth = parseFloat(strW.substring(0, strW.length - 2));
    const visibleHeight = parseFloat(strH.substring(0, strH.length - 2));
    return [visibleWidth, visibleHeight];
  }

  private _mouseOffsetToCanvasPoint(e: MouseEvent): Point {
    // The element changes size occasionally, compute visible size just before use.
    const size = this._visibleSize();
    return [
      Math.round((e.offsetX / size[0]) * this._width),
      Math.round((e.offsetY / size[1]) * this._height),
    ];
  }

  private _sendCursorMove(p: Point) {
    this.dispatchEvent(
      new CustomEvent<CursorEventDetail>(
        MoveCursorEvent, {
          detail: { position: p, onlyData: false },
          bubbles: true,
        },
      ),
    );
  }

  private _drawCrossHairAt(p: Point) {
    const chCanvas = this.querySelector<HTMLCanvasElement>('#crosshair-canvas')!;
    const chx = chCanvas.getContext('2d')!;
    chx.clearRect(0, 0, chCanvas.width, chCanvas.height);

    chx.lineWidth = this._width / this._visibleSize()[0];
    chx.strokeStyle = '#F00';
    chx.beginPath();
    chx.moveTo(0, p[1] - 0.5);
    chx.lineTo(chCanvas.width + 1, p[1] - 0.5);
    chx.moveTo(p[0] - 0.5, 0);
    chx.lineTo(p[0] - 0.5, chCanvas.height + 1);
    chx.stroke();
  }

  private _canvasClicked(e: MouseEvent) {
    if (e.offsetX < 0) { return; } // border
    const coords = this._mouseOffsetToCanvasPoint(e);
    if (this._crossHairActive) {
      this._crossHairActive = false;
      this._drawCrossHairAt([-5, -5]); // lazy clear
      this._sendCursorMove(coords);
    } else {
      this._crossHairActive = true;
      this._sendCursorMove(coords);
    }
  }

  private _canvasMouseMove(e: MouseEvent) {
    if (e.offsetX < 0) { return; } // border
    if (this._crossHairActive) { return; }
    this._sendCursorMove(this._mouseOffsetToCanvasPoint(e));
  }
}

define('debug-view-sk', DebugViewSk);
