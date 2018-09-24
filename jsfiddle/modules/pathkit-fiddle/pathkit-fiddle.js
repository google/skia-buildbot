import 'elements-sk/error-toast-sk'
import { html } from 'lit-html'

import { WasmFiddle, codeEditor } from '../wasm-fiddle'

const PathKitInit = require('pathkit-wasm/bin/pathkit.js');


// Main template for this element
const template = (ele) => html`
<header>
  <div class=title>PathKit Fiddle</div>
  <div class=flex></div>
  <div class=version>PathKit Version: <a href="https://www.npmjs.com/package/pathkit-wasm">0.4.0</a></div>
</header>

<main>
  ${codeEditor(ele)}
  <div class=output>
    <div class=buttons>
      <button class=action @click=${() => ele.run()}>Run</button>
      <button class=action @click=${() => ele.save()}>Save</button>
    </div>
    <div id=canvasContainer><canvas width=500 height=500></canvas></div>
  </div>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>`;

/**
 * @module jsfiddle/modules/canvaskit-fiddle
 * @description <h2><code>canvaskit-fiddle</code></h2>
 *
 * <p>
 *   The top level element for displaying canvaskit fiddles.
 *   The main elements are a code editor box (textarea), a canvas
 *   on which to render the result and a few buttons.
 * </p>
 *
 */
window.customElements.define('pathkit-fiddle', class extends WasmFiddle {

  constructor() {
    super(PathKitInit, template, 'PathKit', 'pathkit');
  }

});
