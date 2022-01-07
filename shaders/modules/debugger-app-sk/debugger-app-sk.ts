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

import { Convert, DebugTrace } from '../debug-trace/debug-trace';
import { DebugTracePlayer } from '../debug-trace-player/debug-trace-player';

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
  private trace: DebugTrace | null = null;

  private player: DebugTracePlayer = new DebugTracePlayer();

  private codeMirror: CodeMirror.Editor | null = null;

  private currentLineMarker: CodeMirror.TextMarker | null = null;

  constructor() {
    super(DebuggerAppSk.template);
  }

  private static themeFromCurrentMode(): string {
    return isDarkMode() ? 'ambiance' : 'base16-light';
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    // Set up drag-and-drop support.
    this.enableDragAndDrop($$<HTMLDivElement>('#drag-area', this)!);

    // Set up CodeMirror.
    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'x-shader/x-sksl',
      theme: DebuggerAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
      scrollbarStyle: 'native',
      readOnly: true,
    });
    this.codeMirror!.setValue('/*** Drag in a DebugTrace JSON file to start the debugger. ***/');

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', DebuggerAppSk.themeFromCurrentMode());
    });
  }

  getEditor(): CodeMirror.Editor | null {
      return this.codeMirror;
  }

  updateCurrentLineMarker(): void {
    this.currentLineMarker?.clear();
    this.currentLineMarker = null;

    if (!this.player.traceHasCompleted()) {
      const lineNumber = this.player.getCurrentLine() - 1;  // CodeMirror uses zero-indexed lines
      this.currentLineMarker = this.codeMirror!.markText(
        { line: lineNumber,     ch: 0 },
        { line: lineNumber + 1, ch: 0 },
        { className: 'cm-current-line' },
      );
      this.codeMirror!.scrollIntoView({ line: lineNumber, ch: 0 }, /*margin=*/36);
    }
  }

  loadJSONData(jsonData: string): void {
    try {
      this.trace = Convert.toDebugTrace(jsonData);
      this.player.reset(this.trace);
      this.player.step();
      this.codeMirror!.setValue(this.trace.source.join('\n'));
      this.updateCurrentLineMarker();
      this._render();
    } catch (ex) {
      this.codeMirror!.setValue((ex instanceof Error) ? ex.message : String(ex));
    }
  }

  private enableDragAndDrop(dropArea: HTMLDivElement): void {
    dropArea.addEventListener('dragover', (event: DragEvent) => {
      event.stopPropagation();
      event.preventDefault();
      event.dataTransfer!.dropEffect = 'move';
    });

    dropArea.addEventListener('drop', (event: DragEvent) => {
      event.stopPropagation();
      event.preventDefault();
      const fileList = event.dataTransfer!.files;
      if (fileList.length === 1) {
        const reader = new FileReader();
        reader.addEventListener('load', () => {
          try {
            this.loadJSONData(reader.result as string);
          } catch (ex) {
            this.codeMirror!.setValue('Unable to read JSON trace file.');
          }
        });
        reader.readAsText(fileList[0]);
      }
    });
  }

  step(): void {
    this.player.step();
    this.updateCurrentLineMarker();
  }

  stepOver(): void {
    this.player.stepOver();
    this.updateCurrentLineMarker();
  }

  stepOut(): void {
    this.player.stepOut();
    this.updateCurrentLineMarker();
  }

  resetTrace(): void {
    this.player.reset(this.trace);
    this.player.step();
    this.updateCurrentLineMarker();
  }

  private static template = (self: DebuggerAppSk): TemplateResult => html`
    <div id="drag-area">
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
        <div id=debuggerControls>
          <span id=buttonGroup>
            <button ?disabled=${self.trace === null}
                    @click=${self.resetTrace}
                    class=action>
              Reset
            </button>
          </span>
          <span id=buttonGroup>
            <button ?disabled=${self.trace === null}
                    @click=${self.stepOver}
                    class=action>
              Step
            </button>
            <button ?disabled=${self.trace === null}
                    @click=${self.step}
                    class=action>
              Step In
            </button>
            <button ?disabled=${self.trace === null}
                    @click=${self.stepOut}
                    class=action>
              Step Out
            </button>
          </span>
        </div>
        <br>
        <div id="codeEditor"></div>
      </main>
    </div>
  `;
}

define('debugger-app-sk', DebuggerAppSk);
