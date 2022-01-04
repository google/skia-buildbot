/**
 * @module modules/debugger-app-sk
 * @description <h2><code>debugger-app-sk</code></h2>
 *
 */
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';

import '../../../infra-sk/modules/theme-chooser-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

export class DebuggerAppSk extends ElementSk {
  constructor() {
    super(DebuggerAppSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private static template = (ele: DebuggerAppSk) => html`
    <header>
      <h2>SkSL Debugger</h2>
      <span>
        <a
          id="githash"
          href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
        >
          ${SKIA_VERSION.slice(0, 7)}
        </a>
        <theme-chooser-sk></theme-chooser-sk>
      </span>
    </header>
    <main>
      TODO(skia:12778): implement SkSL web debugger
    </main>
  `;
}

define('debugger-app-sk', DebuggerAppSk);
