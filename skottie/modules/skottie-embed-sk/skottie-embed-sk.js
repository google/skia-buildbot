/**
 * @module skottie/skottie-embed-sk
 * @description <h2><code>skottie-embed-sk</code></h2>
 *
 * Displays just the WASM based animation suitable for iframing.
 *
 * @evt
 *
 * @attr
 *
 * @example
 *
 *  <iframe width=128 height=128
 *    src="https://skottie.skia.org/e/1112d01d28a776d777cebcd0632da15b"
 *    scrolling=no>
 *  </iframe>
 */
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

const DPR = window.devicePixelRatio;

const wasmCanvas = (ele) => html`
<canvas id=skottie width=${ele._state.width * DPR} height=${ele._state.height * DPR}
        style='width: ${ele._state.width}px; height: ${ele._state.height}px;'>
</canvas>`;

const template = (ele) => html`${wasmCanvas(ele)}`;

window.customElements.define('skottie-embed-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: null,
      width: 256,
      height: 256,
      fps: 30,
    };
    this._skAnimation = null;
    this._skCanvas = null;
    this._skSurface = null;
    this._wasmDuration = null;
    this._firstFrameTime = null;
  }

  connectedCallback() {
    this._reflectFromURL();

    CanvasKitInit({
      locateFile: (file) => '/static/'+file,
    }).then((CanvasKit) => {
      this.CanvasKit = CanvasKit;
      this._render();
    });

    this._render();

    // Start a continous animation loop.
    const drawFrame = () => {
      window.requestAnimationFrame(drawFrame);
      if (!this.CanvasKit || !this._state.lottie) {
        return;
      }
      if (!this._skCanvas) {
        this._skSurface = this.CanvasKit.MakeCanvasSurface('skottie');
        if (!this._skSurface) {
          errorMessage('Could not make SkSurface');
          return;
        }
        this._skCanvas = this._skSurface.getCanvas();
        this._skCanvas.scale(DPR, DPR);
      }
      if (!this._skAnimation) {
        this._skAnimation = this.CanvasKit.MakeAnimation(JSON.stringify(this._state.lottie));
        this._wasmDuration = this._skAnimation.duration() * 1000;
        this._firstFrameTime = Date.now();
      }
      let now = Date.now();
      let seek = ((now - this._firstFrameTime) / this._wasmDuration ) % 1.0;
      this._renderSkottieAt(seek);
    }

    window.requestAnimationFrame(drawFrame);
  }

  _renderSkottieAt(seek) {
    if (this._skAnimation && this._skCanvas) {
      this._skAnimation.seek(seek);
      let bounds = {fLeft: 0, fTop: 0, fRight: this._state.width, fBottom: this._state.height};
      this._skAnimation.render(this._skCanvas, bounds);
      this._skCanvas.flush();
    }
  }

  _reflectFromURL() {
    // Check URL.
    let match = window.location.pathname.match(/\/e\/([a-zA-Z0-9]+)/);
    if (!match) {
      // Make this the hash of the lottie file you want to play on startup.
      this._hash = '1112d01d28a776d777cebcd0632da15b'; // gear.json
    } else {
      this._hash = match[1];
    }
    this._render();
    // Run this on the next micro-task to allow mocks to be set up if needed.
    setTimeout(() => {
      fetch(`/_/j/${this._hash}`, {
        credentials: 'include',
      }).then(jsonOrThrow).then(json => {
        this._state = json;
        this._render();
      }).catch((msg) => {
        console.log(msg);
        this._render();
      });
    });
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
