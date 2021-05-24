import 'elements-sk/error-toast-sk';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { SKIA_VERSION } from '../../build/version';
import {
  WasmFiddle,
} from '../wasm-fiddle/wasm-fiddle';

import '../../../infra-sk/modules/theme-chooser-sk';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const PathKitInit = require('../../build/pathkit/pathkit.js');

// Main template for this element
const template = (ele: WasmFiddle) => html`
<header>
  <div class=title>PathKit Fiddle</div>
  <div class=npm>
    <a href="https://www.npmjs.com/package/pathkit-wasm">Available on npm</a>
  </div>
  <div class=flex></div>

  <div class=version>
    <a href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}">${SKIA_VERSION.substring(0, 10)}</a>
  </div>
  <theme-chooser-sk dark></theme-chooser-sk>
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
</footer>`;

const wasmPromise = PathKitInit({
  locateFile: (file: string) => `/res/${file}`,
});

/**
 * @module jsfiddle/modules/pathkit-fiddle
 * @description <h2><code>pathkit-fiddle</code></h2>
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

define('pathkit-fiddle', PathKitFiddle);
