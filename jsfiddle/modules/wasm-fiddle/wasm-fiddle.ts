import 'elements-sk/styles/buttons';

import 'codemirror/mode/javascript/javascript'; // Syntax highlighting for js.
import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { html, render, TemplateResult } from 'lit-html';
import CodeMirror from 'codemirror';
import { FPS } from '../../../infra-sk/modules/fps/fps';
import 'elements-sk/styles/buttons';
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.

import type {
  CanvasKit,
} from '../../build/canvaskit/canvaskit.js';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

/** Regexp to determine if the code measures FPS. */
const fpsRegex = /benchmarkFPS/;
export const sliderRegex = /#slider(\d):(\S+)/g; // Exported for tests.
export const colorPickerRegex = /#color(\d):(\S+)/g; // Exported for tests.
/**
 * PathKit doesn't export TypeScript interfaces like CanvasKit does, so we use
 * 'any' for now. */
type PathKit = any;

/** What users call the library e.g. 'CanvasKit' */
type LibraryName = 'CanvasKit' | 'PathKit'

/** The backend name for the fiddle e.g. 'canvasKit'. */
type FiddleType = 'canvaskit' | 'pathkit'

/** Uses the regular expression to pull out either the slider or color pickers
 * from the code. */
export const extractControlNames = (r: RegExp, s: string): string[] => {
  const ret: string[] = [];
  let match: string[] = [];
  // eslint-disable-next-line no-cond-assign
  while ((match = r.exec(s) || []).length > 0) {
    ret[+match[1]] = match[2];
  }
  return ret;
};

/**
 * Base class for the PathKit and CanvasKit elements, an element that has a code
 * editor and canvas on which to display the output of a WASM-based library.
 *
 * Assumptions:
 *
 * - The main template makes a call to WasmFiddle.codeEditor() and has a
 *   <div> element with id 'canvasContainer' where the canvas
 *   should go.
 * - There are buttons that call run() and save() on 'this'.
 * - The foo.wasm has been copied into /res/ by means of CopyWebpackPlugin.
 *
 */
export class WasmFiddle extends HTMLElement {
  /** The JS code being run. */
  _content: string = '';

  Wasm: CanvasKit | PathKit | null = null;

  wasmPromise: Promise<CanvasKit | PathKit>;

  editor: CodeMirror.Editor | null = null;

  templateFunc: (ele: WasmFiddle)=> TemplateResult;

  libraryName: LibraryName;

  fiddleType: FiddleType;

  hasRun: boolean = false;

  loadedWasm: boolean = false;

  sliders: string[] = []; // The display names of the sliders.

  colorpickers: string[] = []; // The display names of the color pickers.

  fpsMeter: boolean = false; // True if the supplied template has an FPS meter.

  /**
   * This will be updated to have any captured console.log (but not console.error or console.warn)
   * messages. this._render will be called on any updates to log as well.
   */
  log: string = '';

  /**
   * runID is a unique identifier that changes every time run is clicked. This allows the client
   * code to stop animation loops when run is clicked a second time. See _activeRunInstance().
   */
  runID: number = 0;

  /**
    * @param wasmPromise: Promise that will resolve with the WASM library.
    * @param templateFunc: The base template for this element.
    * @param libraryName: What users call the library e.g. 'CanvasKit'
    * @param fiddleType: The backend name for the fiddle e.g. 'canvasKit'
    */
  constructor(wasmPromise: Promise<CanvasKit | PathKit>, templateFunc: (ele: WasmFiddle)=> TemplateResult, libraryName: LibraryName, fiddleType: FiddleType) {
    super();

    this.wasmPromise = wasmPromise;
    this.templateFunc = templateFunc;
    this.libraryName = libraryName; // e.g. 'CanvasKit' , 'PathKit'
    this.fiddleType = fiddleType; // e.g. 'canvaskit', 'pathkit'
  }

  /**
   * Returns the number of lines in str, with a minimum of 10 (because the
   * editor with less than 10 lines looks a bit strange). See
   * https://stackoverflow.com/a/4009768
   */
  static lines = (str: string): number => Math.max(10, (str.match(/\n/g) || []).length + 1)

  /**
   * repeat returns an array of n 'undefined' which allows for repeating a
   * template a fixed number of times  using map. See
   * https://stackoverflow.com/a/10050831
   */
  static repeat = (n: number): any[] => [...Array(n)]

  static lineNumber = (n: number): TemplateResult => html`<div id=${`L${n}`}>${n}</div>`;

  static codeEditor = (ele: WasmFiddle): TemplateResult => html`<div id=editor></div>`

  static floatSlider = (name: string, i: number): TemplateResult => {
    if (!name) {
      return html``;
    }
    // By setting the input's name=sliderN, the JS function will magically have a global variable
    // called sliderN that refers to the input HTML element.
    // https://www.w3schools.com/tags/att_input_name.asp
    return html`<div class="widget">
      <input
        name=${`slider${i}`}
        id=${`slider${i}`}
        min="0"
        max="1"
        step="0.00001"
        type="range"
      />
      <label for=${`slider${i}`}>${name}</label>
    </div>`;
  }

  static colorPicker = (name: string, i: number): TemplateResult => {
    if (!name) {
      return html``;
    }
    // By setting the input's name=colorN, the JS function will magically have a global variable
    // called colorN that refers to the input HTML element.
    // https://www.w3schools.com/tags/att_input_name.asp
    return html` <div class="widget">
      <input name=${`color${i}`} id=${`color${i}`} type="color" />
      <label for=${`color${i}`}>${name}</label>
    </div>`;
  }

  /** @prop The current code in the editor. */
  get content(): string {
    return this._content;
  }

  set content(c: string) {
    this._content = c;
    this.enumerateWidgets();
    this._render();
    // Avoid infinite recursion.
    if (c !== this.editor!.getValue()) {
      this.editor!.setValue(c);
    }
  }

  /** Returns the CodeMirror theme based on the state of the page's darkmode.
   *
   * For this to work the associated CSS themes must be loaded. See
   * wasm-fiddle.scss.
   */
  private static themeFromCurrentMode = () => (isDarkMode() ? 'ambiance' : 'base16-light');

  connectedCallback(): void {
    // Allows demo pages to supply content w/o making a network request
    this._content = this.getAttribute('content') || '';

    this._render();
    this.editor = CodeMirror($$<HTMLDivElement>('#editor', this)!, {
      lineNumbers: true,
      theme: WasmFiddle.themeFromCurrentMode(),
      viewportMargin: Infinity,
      scrollbarStyle: 'native',
      mode: 'javascript',
    });
    this.editor.on('change', () => this.changed());
    document.addEventListener('theme-chooser-toggle', () => {
      this.editor!.setOption('theme', WasmFiddle.themeFromCurrentMode());
    });

    this.wasmPromise.then((LoadedWasm) => {
      this.Wasm = LoadedWasm;
      this.loadedWasm = true;
      this._render();
    });

    if (!this.content) {
      this.loadCode();
    }
    // Listen for the forward and back buttons and re-load the code
    // on any changes. Without this, the url changes, but nothing
    // happens in the DOM.
    window.addEventListener('popstate', this.loadCode.bind(this));
  }

  disconnectedCallback(): void {
    window.removeEventListener('popstate', this.loadCode.bind(this));
  }

  /** Runs the code, allowing the user to see the result on the canvas. */
  run(): void {
    this.runID = Date.now();
    // reset the log on each run.
    this.log = '';
    // consoleInterceptor is used to intercept console.log calls and store them.
    const consoleInterceptor = {
      log: (...rest: any[]) => {
        // pipe this through to regular console.log
        // eslint-disable-next-line no-console
        console.log(...rest);
        // stringify all the arguments for rendering using the log property.
        for (let i = 0; i < rest.length; i++) {
          const a = rest[i];
          if (typeof a === 'object') {
            // Make an attempt to prettify objects - this doesn't work well on WASM objects
            // or DOMElements.
            this.log += JSON.stringify(a);
          } else {
            this.log += a;
          }
          this.log += ' ';
        }
        this.log += '\n';
        this._render();
      },
      // eslint-disable-next-line no-console
      warn: console.warn,
      // eslint-disable-next-line no-console
      error: console.error,
    };

    if (!this.Wasm) {
      errorMessage(`${this.libraryName} is still loading. Try again in a few seconds.`);
      return;
    }
    this.hasRun = true;

    this._render();
    const canvas = this.resetCanvas();

    try {
      // Because of the magic of setting <input name=sliderN>, we don't need to declare any
      // variables for sliders or colorpickers (see floatSlider and colorPicker above).
      // eslint-disable-next-line no-new-func
      const f = new Function(
        this.libraryName, // e.g. "CanvasKit", the name of the WASM library.
        'canvas', // We provide the canvas element to the user as a parameter named 'canvas'.
        'console', // By having this parameter named 'console', we intercept a user's normal
        // calls to the window.console object [unless they happen to actually say
        // window.console.log('foo')].
        'benchmarkFPS', // provide a helper that the user can call to get an FPS output.
        'isRunning', // provide a helper for the user to stop their animation when run is clicked.
        this.content, // user provided code, as a string, which will be interpreted and executed.
      );
      f(this.Wasm, canvas, consoleInterceptor, this._benchmarkFPSInstance(this.runID),
        this.activeRunInstance(this.runID));
    } catch (e) {
      errorMessage(e);
    }
  }

  /**
    * Sends the code to the backend to be saved. Updates the URL upon success
    * to the new permalink for this fiddle.
    */
  save(): void {
    fetch('/_/save', {
      method: 'PUT',
      headers: new Headers({
        'content-type': 'application/json',
      }),
      body: JSON.stringify({
        code: this.content,
        type: this.fiddleType,
      }),
    }).then(jsonOrThrow).then((json) => {
      window.history.pushState(null, '', json.new_url);
    }).catch(errorMessage);
  }

  // create a brand new canvas. Without this, the context can get muddled
  // between calls, especially when using WebGL. We can't simply drawRect
  // to clear it because that creates a 2d drawing context which prevents
  // use with a webGL context.
  private resetCanvas(): HTMLCanvasElement {
    const cc = $$('#canvasContainer', this);
    cc!.innerHTML = '';
    const canvas = document.createElement('canvas');
    canvas.width = 500;
    canvas.height = 500;
    canvas.id = 'canvas';
    cc!.appendChild(canvas);
    return canvas;
  }

  private _render(): void {
    render(this.templateFunc(this), this, { eventContext: this });
  }

  private changed(): void {
    this.content = this.editor!.getValue();
  }

  // Look through the current source code for references to sliders or colorpickers.
  // These have the magic values #sliderN:displayName and #colorN:displayName and we just
  // search the code given to use with two regex.
  private enumerateWidgets(): void {
    this.sliders = extractControlNames(sliderRegex, this.content);
    this.colorpickers = extractControlNames(colorPickerRegex, this.content);
    this.fpsMeter = !!this.content.match(fpsRegex);
  }

  private loadCode(): void {
    // The location should be either /<fiddleType> or /<fiddleType>/<fiddlehash>
    const path = window.location.pathname;
    let hash = '';
    const len = this.fiddleType.length + 2; // count of chars in /<fiddleType>/
    if (path.length > len) {
      hash = path.slice(len);
    }

    fetch(`/_/code?type=${this.fiddleType}&hash=${hash}`)
      .then(jsonOrThrow)
      .then((json) => {
        this.content = json.code;
      })
      .catch(() => {
        errorMessage('Fiddle not Found', 10000);
        this.content = '';
        this.resetCanvas();
      });
  }

  // Returns a helper function that will return true if the current running instance is the most
  // recent instance. This can be used by client code to stop their animation loops when the Run
  // button is hit again.
  private activeRunInstance(currentRunID: number) {
    return (): boolean => currentRunID === this.runID;
  }

  // Returns a helper function that will store the last 10 frame times and every tenth frame will
  // output the average FPS from those ten frames. The returned function has its variables tied up
  // in a closure so that new instances will not conflict with each other (e.g. when run is clicked)
  // It also checks to see if this invocation is the latest and will do nothing if it is not (e.g.
  // prevent competing updates to the fps meter.
  private _benchmarkFPSInstance(currentRunID: number): ()=> void {
    const fps = new FPS();
    let fpsEle: HTMLElement | null = null;
    return (): void => {
      if (this.runID !== currentRunID) {
        return;
      }
      fps.raf();
      if (!fpsEle) {
        fpsEle = $$('#fps')!;
      }
      fpsEle.textContent = `${fps.fps.toFixed(1)} FPS`;
    };
  }
}
