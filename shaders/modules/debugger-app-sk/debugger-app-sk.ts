/**
 * @module modules/debugger-app-sk
 * @description <h2><code>debugger-app-sk</code></h2>
 *
 */
import { $$ } from 'common-sk/modules/dom';
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import CodeMirror, { EditorConfiguration } from 'codemirror';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { classMap } from 'lit-html/directives/class-map';

import '../../../infra-sk/modules/theme-chooser-sk';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import * as SkSLConstants from '../sksl-constants/sksl-constants';

import { Convert, DebugTrace } from '../debug-trace/debug-trace';
import { DebugTracePlayer, VariableData } from '../debug-trace-player/debug-trace-player';
import '../../../infra-sk/modules/app-sk';

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

enum ErrorReporting {
  Yes = 1,
  No = 0
}

export class DebuggerAppSk extends ElementSk {
  private trace: DebugTrace | null = null;

  private player: DebugTracePlayer = new DebugTracePlayer();

  private codeMirror: CodeMirror.Editor | null = null;

  private currentLineHandle: CodeMirror.LineHandle | null = null;

  private currentHoveredWord: string = '';

  private currentStackFrame: number = -1; // -1 corresponds to global scope

  private currentLineNumber: number = 0; // 0 corresponds to no current line

  private localStorage: Storage = window.localStorage; // can be overridden in tests

  private queryParameter: string = window.location.search; // can be overridden in tests

  constructor() {
    super(DebuggerAppSk.template);
  }

  setLocalStorageForTest(mockStorage: Storage): void {
    this.localStorage = mockStorage;
  }

  setQueryParameterForTest(overrideQueryParam: string): void {
    this.queryParameter = overrideQueryParam;
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
    const editorDiv: HTMLDivElement = $$<HTMLDivElement>('#codeEditor', this)!;
    this.codeMirror = CodeMirror(editorDiv, <EditorConfiguration>{
      value: '/*** Drag in a DebugTrace JSON file to start the debugger. ***/',
      lineNumbers: true,
      mode: 'x-shader/x-sksl',
      theme: DebuggerAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
      scrollbarStyle: 'native',
      readOnly: true,
      gutters: ['CodeMirror-linenumbers', 'cm-breakpoints'],
      fixedGutter: true,
    });
    this.codeMirror!.on('gutterClick', (_, line: number) => {
      // 'line' comes from CodeMirror so is indexed starting from zero.
      this.toggleBreakpoint(line + 1);
    });
    editorDiv.addEventListener('mousemove', (e: MouseEvent) => {
      const mousePos = {left:e.pageX, top:e.pageY};
      const codePos = this.codeMirror!.coordsChar(mousePos);
      const word = this.codeMirror!.findWordAt(codePos);
      const hoveredWord: string = this.codeMirror!.getRange(word.anchor, word.head);
      if (hoveredWord != this.currentHoveredWord) {
        this.currentHoveredWord = hoveredWord;
        this._render();
      }
    });

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', DebuggerAppSk.themeFromCurrentMode());
    });

    // If ?local-storage(=anything), try loading a debug trace from local storage.
    const params = new URLSearchParams(this.queryParameter);
    if (params.has('local-storage')) {
      this.loadJSONData(this.localStorage.getItem('sksl-debug-trace')!, ErrorReporting.No);

      // Remove ?local-storage from the query parameters on the window, so a reload or copy-paste
      // will present a clean slate.
      const url = new URL(window.location.toString());
      url.searchParams.delete('local-storage');
      window.history.pushState({}, '', url.toString());
    }
  }

  getEditor(): CodeMirror.Editor | null {
    return this.codeMirror;
  }

  private updateControls(): void {
    this.currentStackFrame = this.player.getStackDepth() - 1;
    this.currentLineNumber = this.player.getCurrentLine();
    this.updateCurrentLineMarker();
    this._render();
  }

  private updateCurrentLineMarker(): void {
    if (this.currentLineHandle !== null) {
      this.codeMirror!.removeLineClass(this.currentLineHandle!, 'background', 'cm-current-line');
      this.currentLineHandle = null;
    }

    if (this.currentLineNumber > 0) {
      // Subtract one from the line number because CodeMirror uses zero-indexed lines.
      const lineNumber = this.currentLineNumber - 1;
      this.currentLineHandle = this.codeMirror!.addLineClass(lineNumber, 'background',
                                                            'cm-current-line');
      this.codeMirror!.scrollIntoView({ line: lineNumber, ch: 0 }, /* margin= */36);
    }
  }

  private arrowIfCurrentFrameIs(n: number): string {
    return this.currentStackFrame == n ? '➔ ' : '';
  }

  private stackDisplay(): TemplateResult[] {
    let stack: string[] = [];
    if (this.trace) {
      stack = this.player.getCallStack().map((funcIdx: number, frame: number) =>
        (this.arrowIfCurrentFrameIs(frame) + this.trace!.functions[funcIdx].name)
      );
    }
    stack.unshift(this.arrowIfCurrentFrameIs(-1) + 'global scope');

    const result: TemplateResult[] = stack.map((text: String, index: number) => html`
      <tr><td>
        <a href="javascript:;" @click=${() => this.changeStackFrame(index - 1)}>${text}</a>
      </td></tr>`);
    return result.reverse();
  }

  private changeStackFrame(frame: number): void {
    this.currentLineNumber = (frame >= 0) ? this.player.getCurrentLineInStackFrame(frame) : 0;
    this.currentStackFrame = frame;
    this.updateCurrentLineMarker();
    this._render();
  }

  private varsDisplay(vars: VariableData[]): TemplateResult[] {
    if (this.trace && vars.length > 0) {
      return vars.map((v: VariableData) => {
        const name: string = this.trace!.slots[v.slotIndex].name;
        const componentName: string = name + this.player.getSlotComponentSuffix(v.slotIndex);
        const nameClass = {
          'change-highlight': v.dirty,
          'hover-highlight': (name == this.currentHoveredWord)
        };
        const valueClass = {
          'hover-highlight': (name == this.currentHoveredWord)
        };
        return html`
          <tr>
            <td class=${classMap(nameClass)}>${componentName}</td>
            <td class=${classMap(valueClass)}>${v.value}</td>
          </tr>`;
      });
    }
    return [html`<tr><td>&nbsp;</td></tr>`];
  }

  private localVarsDisplay(): TemplateResult[] {
    if (this.currentStackFrame < 0) {
      return [];
    }
    return this.varsDisplay(this.player.getLocalVariables(this.currentStackFrame));
  }

  private globalVarsDisplay(): TemplateResult[] {
    return this.varsDisplay(this.player.getGlobalVariables());
  }

  loadJSONData(jsonData: string, reportErrors?: ErrorReporting): void {
    try {
      this.trace = Convert.toDebugTrace(jsonData);
      this.codeMirror!.setValue(this.trace.source.join('\n'));
      this.player.setBreakpoints(new Set());
      this.resetTrace();
      this.resetBreakpointGutter();
      this._render();
    } catch (ex) {
      if (reportErrors ?? ErrorReporting.Yes) {
        this.codeMirror!.setValue((ex instanceof Error) ? ex.message : String(ex));
      }
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
    this.updateControls();
  }

  stepOver(): void {
    this.player.stepOver();
    this.updateControls();
  }

  stepOut(): void {
    this.player.stepOut();
    this.updateControls();
  }

  run(): void {
    this.player.run();
    this.updateControls();
  }

  resetTrace(): void {
    this.player.reset(this.trace);
    this.player.step();
    this.updateControls();
  }

  private static makeDivWithClass(name: string): HTMLDivElement {
    const marker: HTMLDivElement = document.createElement('div');
    marker.classList.add(name);
    return marker;
  }

  private resetBreakpointGutter(): void {
    this.codeMirror!.clearGutter('cm-breakpoints');
    this.player.getLineNumbersReached().forEach((timesReached: number, line: number) => {
      this.codeMirror!.setGutterMarker(line - 1, 'cm-breakpoints',
        DebuggerAppSk.makeDivWithClass('cm-reachable'));
    });
  }

  toggleBreakpoint(line: number): void {
    // The line number is 1-indexed.
    if (this.player.getBreakpoints().has(line)) {
      this.player.removeBreakpoint(line);
      this.codeMirror!.setGutterMarker(line - 1, 'cm-breakpoints',
        DebuggerAppSk.makeDivWithClass('cm-reachable'));
    } else if (this.player.getLineNumbersReached().has(line)) {
      this.player.addBreakpoint(line);
      this.codeMirror!.setGutterMarker(line - 1, 'cm-breakpoints',
        DebuggerAppSk.makeDivWithClass('cm-breakpoint'));
    } else {
      // Don't allow breakpoints to be set on unreachable lines.
    }

    this._render();
  }

  private static template = (self: DebuggerAppSk): TemplateResult => html`
    <app-sk id="drag-area">
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
                    @click=${self.resetTrace}>
              Reset
            </button>
          </span>
          <span id=buttonGroup>
            <button ?disabled=${self.trace === null}
                    @click=${self.stepOver}>
              Step
            </button>
            <button ?disabled=${self.trace === null}
                    @click=${self.step}>
              Step In
            </button>
            <button ?disabled=${self.trace === null}
                    @click=${self.stepOut}>
              Step Out
            </button>
          </span>
          <span id=buttonGroup>
            <button ?disabled=${self.trace === null}
                    @click=${self.run}>
              ${self.player.getBreakpoints().size > 0 ? 'Run to Breakpoint' : 'Run'}
            </button>
          </span>
        </div>
        <br>
        <div id="debuggerPane">
          <div id="codeEditor"></div>
          <div id="debuggerTables">
            <table>
              <tr><td class="heading">Stack</td></tr>
              ${self.stackDisplay()}
            </table>
            <table>
              <tr ?hidden=${self.currentStackFrame < 0}>
                <td class="heading" colspan=2>Local Variables</td>
              </tr>
              ${self.localVarsDisplay()}
              <tr>
                <td class="heading" colspan=2>Global Variables</td>
              </tr>
              ${self.globalVarsDisplay()}
            </table>
          </div>
        </div>
      </main>
    </app-sk>
  `;
}

define('debugger-app-sk', DebuggerAppSk);
