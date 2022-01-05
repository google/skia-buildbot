/**
 * @module modules/debugger-app-sk
 * @description <h2><code>debugger-app-sk</code></h2>
 *
 */
import { $$ } from 'common-sk/modules/dom';
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import CodeMirror from 'codemirror';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';

import '../../../infra-sk/modules/theme-chooser-sk';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import * as SkSLConstants from '../sksl-constants/sksl-constants';

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

// Define a new mode and mime-type for SkSL shaders. We follow the shader naming
// convention found in CodeMirror.
CodeMirror.defineMIME('x-shader/x-sksl', {
  name: 'clike',
  keywords: SkSLConstants.keywords,
  types: SkSLConstants.types,
  builtin: SkSLConstants.builtins,
  blockKeywords: SkSLConstants.blockKeywords,
  defKeywords: SkSLConstants.defKeywords,
  typeFirstDefinitions: true,
  atoms: SkSLConstants.atoms,
  modeProps: { fold: ['brace', 'include'] },
});

export class DebuggerAppSk extends ElementSk {
  private codeMirror: CodeMirror.Editor | null = null;

  constructor() {
    super(DebuggerAppSk.template);
  }

  private static themeFromCurrentMode(): string {
    return isDarkMode() ? 'ambiance' : 'base16-light';
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    // Set up CodeMirror.
    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'x-shader/x-sksl',
      theme: DebuggerAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
      scrollbarStyle: 'native',
      readOnly: true
    });
    this.codeMirror!.setValue(String.raw`// TODO(skia:12778): implement SkSL web debugger

half4 main(float2 p) {
  return half4(p.xy01);
}`);

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', DebuggerAppSk.themeFromCurrentMode());
    });
  }

  private static template = (ele: DebuggerAppSk): TemplateResult => html`
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
      <div id="codeEditor"></div>
    </main>
  `;
}

define('debugger-app-sk', DebuggerAppSk);
