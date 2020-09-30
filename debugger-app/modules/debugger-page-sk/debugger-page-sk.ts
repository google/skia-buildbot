/**
 * @module modules/debugger-page-sk
 * @description <h2><code>debugger-page-sk</code></h2>
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
//import { error-toast-sk } from '../../../elements-sk/modules/error-toast-sk';
// other modules from this application
// import '../multi-frame-controls-sk';
// import '../filter-sk';
// import '../play-sk';
// import '../commands-sk';
// import '../debug-view-sk';
import '../matrix-clip-controls-sk';
// import '../command-histogram-sk';
// import '../zoom-sk';
// import '../android-layers-sk';


export class DebuggerPageSk extends ElementSk {
  private static template = (ele: DebuggerPageSk) =>
    html`
    <header class="horizontal layout center">
      <h2>Skia WASM Debugger</h2>
    </header>
    <div id=content>
      <div class="horizontal">
        <label>SKP to open:</label>
        <input type="file" id="file_input" disabled />
        <p>File version ${ele.fileVersion}</p>
      </div>
      <multi-frame-controls-sk class="horizontal"></multi-frame-controls-sk>
      <div class="horizontal">
        <div id=left class="vertical"><!--
          <filter-sk id=filter class="horizontal"></filter-sk>
          <play-sk id=play class="horizontal"></play-sk>
          <commands-sk id=commands class="horizontal"></commands-sk>-->
        </div>
        <div id=center class="vertical">
          <debug-view-sk></debug-view-sk>
        </div>
        <div id=right class="vertical">
          <matrix-clip-controls-sk class="horizontal"></matrix-clip-controls-sk>
          <!-- hidable gpu op bounds legend-->
          <command-histogram-sk class="horizontal"></command-histogram-sk>
          <!-- cursor position and color -->
          <!-- breakpoint controls -->
          <!-- hidable gpu op bounds legend-->
          <zoom-sk class="horizontal"></zoom-sk>
          <!-- keyboard shortcuts legend -->
          <android-layers-sk class="horizontal"></android-layers-sk>
        </div>
      </div>
    </div>
    <error-toast-sk></error-toast-sk>
    `;

  fileVersion: string;

  constructor() {
    super(DebuggerPageSk.template);
    this.fileVersion = 'local'
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('debugger-page-sk', DebuggerPageSk);
