import 'elements-sk/error-toast-sk';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { WasmFiddle } from '../wasm-fiddle-sk/wasm-fiddle-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

// It is assumed that pathkit.js has been loaded and this symbol is available globally.
declare const PathKitInit: any;

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

// Main template for this element
const template = (ele: WasmFiddle) => html`
<app-sk>
  <header>
    <h1>PathKit Fiddle</h1>
    <div>
      <a href="https://www.npmjs.com/package/pathkit-wasm">Available on npm</a>
      <a href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}">${SKIA_VERSION.substring(0, 10)}</a>
      <theme-chooser-sk></theme-chooser-sk>
    </div>
  </header>

  <main>
    ${WasmFiddle.codeEditor(ele)}
    <div class=output>
      ${ele.sliders.map(WasmFiddle.floatSlider)}
      ${ele.colorpickers.map(WasmFiddle.colorPicker)}
      ${ele.fpsMeter ? html`<div class=widget id=fps>0 FPS</div>` : ''}
      <div class=buttons>
        <button class="action ${(ele.hasRun || !ele.loadedWasm) ? '' : 'prompt'}" @click=${ele.run}>Run</button>
        <button @click=${ele.save}>Save</button>
      </div>
      <div id=canvasContainer><canvas width=500 height=500></canvas></div>
      <textarea id=logsContainer placeholder="Console Logs" readonly>${ele.log}</textarea>
    </div>
  </main>
  <footer>
    <error-toast-sk></error-toast-sk>
  </footer>
</app-sk>`;

const wasmPromise = PathKitInit({
  locateFile: (file: string) => `/res/${file}`,
});

/**
 * @module jsfiddle/modules/pathkit-fiddle-sk
 * @description <h2><code>pathkit-fiddle-sk</code></h2>
 *
 * <p>
 *   The top level element for displaying pathkit fiddles.
 *   The main elements are a code editor box (textarea), a canvas
 *   on which to render the result and a few buttons.
 * </p>
 *
 */

class PathKitFiddle extends WasmFiddle {
  constructor() {
    super(wasmPromise, template, 'PathKit', 'pathkit');
  }
}

define('pathkit-fiddle-sk', PathKitFiddle);
