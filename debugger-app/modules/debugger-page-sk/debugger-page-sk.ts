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
import '../debug-view-sk';
import '../matrix-clip-controls-sk';
// import '../command-histogram-sk';
// import '../zoom-sk';
// import '../android-layers-sk';


export class DebuggerPageSk extends ElementSk {
  private static template = (ele: DebuggerPageSk) =>
    html`
    <header>
      <h2>Skia WASM Debugger</h2>
      <a id="version-link" href='https://skia.googlesource.com/skia/+show/${ele.skiaVersion}'
         title="The skia commit at which the debugger WASM module was built">
        ${ele.skiaVersionShort}
      </a>
    </header>
    <div id=content>
      <div class="horizontal-flex">
        <label>SKP to open:</label>
        <input type="file" id="file-input" disabled />
        <p>File version ${ele.fileVersion}</p>
      </div>
      <multi-frame-controls-sk id></multi-frame-controls-sk>
      <div class="horizontal-flex">
        <div>
          <filter-sk id=filter></filter-sk>
          <play-sk id=play></play-sk>
          <commands-sk id=commands></commands-sk>
        </div>
        <div id=center>
          <debug-view-sk></debug-view-sk>
        </div>
        <div id=right>
          <matrix-clip-controls-sk style="width: 20em"></matrix-clip-controls-sk>
          <!-- hidable gpu op bounds legend-->
          <command-histogram-sk></command-histogram-sk>
          <!-- cursor position and color -->
          <!-- breakpoint controls -->
          <!-- hidable gpu op bounds legend-->
          <zoom-sk></zoom-sk>
          <!-- keyboard shortcuts legend -->
          <android-layers-sk></android-layers-sk>
        </div>
      </div>
    </div>
    <error-toast-sk></error-toast-sk>
    `;

  fileVersion: string;
  skiaVersion: string;
  skiaVersionShort: string;

  constructor() {
    super(DebuggerPageSk.template);
    this.fileVersion = '-77'
    this.skiaVersion = 'a url of some kind'
    this.skiaVersionShort = '-88'
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('debugger-page-sk', DebuggerPageSk);
