/**
 * @module modules/zoom-sk
 * @description A module that shows a zoomed in view of the canvas
 * Like commands-sk and histogram, the zoom module is another case of data
 * (a cursor location) that can be viewed and controlled from two modules.
 *
 * The zoom module shows the cursor location by where it sources data from
 * its source canvas, and allows the cursor to be moved by clicking on the zoom
 * canvas or by key bindings
 *
 * The crosshair (which is part of debug-view-sk) also shows the cursor location
 * and allows it to be moved. The cursor location is owned by zoom-sk, and
 * communicated to debug-view-sk via events.
 *
 * The zoom element also contains a textural readout of the cursor position
 * and color of the selected pixel.
 *
 * @evt move-cursor emitted when the user changes the cursor position by clicking
 *   the zoom view. The position is a coordinate in the source canvas.
 *   See debugger-page-sk for more info on move-cursor and render-cursor
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementDocSk } from '../element-doc-sk/element-doc-sk';
import {
  DebuggerPageSkLightDarkEventDetail,
  DebuggerPageSkCursorEventDetail,
  Point,
} from '../debugger-page-sk/debugger-page-sk';

function clamp(c: number): number {
  return Math.round(Math.max(0, Math.min(c || 0, 255)));
}

export class ZoomSk extends ElementDocSk {
  private static template = (ele: ZoomSk) =>
    html`
<dl>
  <dt><b>Postion</b></dt>
  <dd>(${ele._cursor[0]}, ${ele._cursor[1]})</dd>
  <dt><b>Color</b></dt>
  <dd>
    <div class=color-preview id=prevColor style="background-color: ${ele._rgb}">
    </div>${ele._rgb}
  </dd>
  <dd>${ele._hex}</dd>
</dl>
<div> <!-- this div is block while the one inside it is inline-block -->
  <div class="${ele._backdropStyle} shrink">
    <canvas class="zoom-canvas" width=228 height=228
      @click=${ele._canvasClicked}></canvas>
  </div>
</div>
<details>
  <summary><b>Keyboard shortcuts</b></summary>
  <table class=shortcuts>
    <tr><th>H</th><td>Cursor left</td></tr>
    <tr><th>L</th><td>Cursor right</td></tr>
    <tr><th>J</th><td>Cursor down</td></tr>
    <tr><th>K</th><td>Cursor up</td></tr>
    <tr><th>.</th><td>Step command forward</td></tr>
    <tr><th>,</th><td>Step command back</td></tr>
    <tr><th>w</th><td>Previous Frame</td></tr>
    <tr><th>s</th><td>Next Frame</td></tr>
    <tr><th>p</th><td>Play/Pause frame playback </td></tr>
    <tr><td colspan=2>Click the image again to turn off keyboard navigation.</td></tr>
  </table>
</details>`;

  // Our own canvas
  private _canvas: HTMLCanvasElement | null = null;
  // The other canvas we are showing a zoomed view of
  private _source: HTMLCanvasElement | null = null;
  // cursor location. origin is top left
  private _cursor: Point = [0, 0];
  // color of the last selected pixel
  private _rgb = '';
  private _hex = '';
  private _backdropStyle = 'light-checkerboard';

  // must be an odd number of pixels
  // view is square, this is width and height
  private static ps = 12; // width of one zoomed pixel
  private static viewSize = 228
  private static size = 19; // * 12x zoom
  private static halfSize = 9;

  constructor() {
    super(ZoomSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this._canvas = this.querySelector<HTMLCanvasElement>('canvas')!;

    this.addDocumentEventListener('render-cursor', (e) => {
      const detail = (e as CustomEvent<DebuggerPageSkCursorEventDetail>).detail;
      // these three steps cannot happen in any other order, hence the repeated condition.
      if (!detail.onlyData) {
        this._cursor = detail.position;
      }
      this.update(); // to draw the canvas from the new cursor
      this._render(); // to update the textual readout of the cursor in the template
    });

    this.addDocumentEventListener('light-dark', (e) => {
      this._backdropStyle = (e as CustomEvent<DebuggerPageSkLightDarkEventDetail>).detail.mode;
      this._render();
    });
  }

  set source(newsource: HTMLCanvasElement) {
    this._source = newsource;
  }

  get point(): Point {
    return this._cursor;
  }

  /** Redraw the zoomed in canvas */
  update() {
    const ctx = this._canvas!.getContext('2d')!;

    // Clears to transparent black. it's important that the checkerboard show through.
    ctx.clearRect(0, 0, this._canvas!.width, this._canvas!.height);

    // html canvas origin is top left.
    const sourcex = this._cursor[0] - ZoomSk.halfSize;
    const sourcey = this._cursor[1] - ZoomSk.halfSize;
    ctx.imageSmoothingEnabled = false;
    ctx.drawImage(this._source!, sourcex, sourcey, ZoomSk.size, ZoomSk.size,
      0, 0, ZoomSk.viewSize, ZoomSk.viewSize);

    // Box one selected pixel in the exact middle of the canvas.
    ctx.strokeRect(ZoomSk.halfSize*ZoomSk.ps+0.5, ZoomSk.halfSize*ZoomSk.ps+0.5,
      ZoomSk.ps, ZoomSk.ps);

    // store the color of the selected pixel.
    // gives a UInt8ClampedArray of RGBA
    const c = ctx.getImageData(ZoomSk.viewSize/2, ZoomSk.viewSize/2, 1, 1).data;
    this._rgb = `rgba(${c[0]}, ${c[1]}, ${c[2]}, ${c[3]})`;
    this._hex = ((
      (clamp(c[0]) << 24) |
      (clamp(c[1]) << 16) |
      (clamp(c[2]) << 8) |
      (clamp(c[3]) << 0) & 0xFFFFFFF) >>> 0).toString(16);
  }

  // convert click in zoomed view to coordinates in source canvas
  // skia origin is top left
  private _canvasClicked(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    const x = Math.floor((e.offsetX-1) / ZoomSk.ps) - ZoomSk.halfSize;
    const y = Math.floor(e.offsetY / ZoomSk.ps) - ZoomSk.halfSize;
    const cx = Math.min(Math.max(this._cursor[0] + x, 0), this._source!.width);
    const cy = Math.min(Math.max(this._cursor[1] + y, 0), this._source!.height);
    // Don't render yet, just send the event, headquarters will tell you when to render.
    // Emit zoom-point
    this.dispatchEvent(
      new CustomEvent<DebuggerPageSkCursorEventDetail>(
        'move-cursor', {
          detail: {position: [cx, cy], onlyData: false},
          bubbles: true,
        }));
  }

};

define('zoom-sk', ZoomSk);
