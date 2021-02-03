/**
 * @module modules/shaders-app-sk
 * @description <h2><code>shaders-app-sk</code></h2>
 *
 */
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk/ElementSk';
import { SKIA_VERSION } from '../../build/version';
import { errorMessage } from 'elements-sk/errorMessage';
import CodeMirror from 'codemirror';
import { $$ } from 'common-sk/modules/dom';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import type {
  CanvasKit,
  Surface,
  Canvas,
  RuntimeEffect,
  Paint,
} from '../../build/canvaskit/canvaskit.js';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import '../../../infra-sk/modules/theme-chooser-sk';
import { HintableObject } from 'common-sk/modules/hintable';
import { ScrapBody, ScrapID } from '../json';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL((document!.currentScript as HTMLScriptElement).src)
  .origin;
const kitReady = CanvasKitInit({
  locateFile: (file: any) => `${scriptOrigin}/dist/${file}`,
});

const DEFAULT_SIZE = 256;

const defaultShader = `uniform float u_time;

half4 main(float2 fragCoord) {
 return vec4(1.0, 0, u_time, 1.0);
}`;

type stateChangedCallback = () => void;

// State represents data reflected to/from the URL.
interface State {
  id: string;
}

const defaultState: State = {
  id: '',
};

// requestAnimationFrame id if requestAnimationFrame is not running.
const RAF_NOT_RUNNING = -1;

export class ShadersAppSk extends ElementSk {
  private codeMirror: CodeMirror.Editor | null = null;

  private canvasEle: HTMLCanvasElement | null = null;

  private kit: CanvasKit | null = null;

  private canvasKitContext: number = -1;

  private surface: Surface | null = null;

  private canvas: Canvas | null = null;

  private paint: Paint | null = null;

  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private duration: number = 2000; // ms

  private startTime: number = 0;

  private effect: RuntimeEffect | null = null;

  private state: State = defaultState;

  // The requestAnimationFrame id if we are running, otherwise we are not running.
  private rafID: number = RAF_NOT_RUNNING;

  // Records the code that we started with, either at startup, or after we've saved.
  private lastSavedCode = defaultShader;

  // Records the code that is currently running.
  private runningCode = defaultShader;

  // The current code in the editor.
  private editedCode = defaultShader;

  // stateReflector update function.
  private stateChanged: stateChangedCallback | null = null;

  constructor() {
    super(ShadersAppSk.template);
  }

  private static template = (ele: ShadersAppSk) => html`
    <header>
      <h2>SkSL Shaders</h2>
      <span>
        <a
          id="githash"
          href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
        >
          ${SKIA_VERSION.slice(0, 7)}
        </a>
        <theme-chooser-sk dark></theme-chooser-sk>
      </span>
    </header>
    <main>
      <canvas
        id="player"
        width=${ele.width * window.devicePixelRatio}
        height=${ele.height * window.devicePixelRatio}
        style="width: ${ele.width}px; height: ${ele.height}px;"
      >
        Your browser does not support the canvas tag.
      </canvas>
      <div id="codeEditor"></div>
      <div>
        <button
          ?hidden=${ele.editedCode === ele.runningCode}
          @click=${ele.runClick}
        >
          Run
        </button>
        <button
          ?hidden=${ele.editedCode === ele.lastSavedCode}
          @click=${ele.saveClick}
        >
          Save
        </button>
      </div>
    </main>
    <footer>
      <error-toast-sk></error-toast-sk>
    </footer>
  `;

  /** Returns the CodeMirror theme based on the state of the page's darkmode.
   *
   * For this to work the associated CSS themes must be loaded. See
   * shaders-app-sk.scss.
   */
  private static themeFromCurrentMode = () => {
    return isDarkMode() ? 'base16-dark' : 'base16-light';
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.startTime = Date.now();
    this.canvasEle = $$<HTMLCanvasElement>('#player', this);
    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'text/x-c++src',
      theme: ShadersAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
    });
    this.codeMirror.on('change', () => this.codeChange());

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', ShadersAppSk.themeFromCurrentMode());
    });

    // Continue the setup once CanvasKit WASM has loaded.
    kitReady.then((ck: CanvasKit) => {
      this.kit = ck;
      this.paint = new this.kit!.Paint();
      try {
        this.stateChanged = stateReflector(
          /* getState */ () => (this.state as unknown) as HintableObject,
          /* setState */ (newState: HintableObject) => {
            this.state = (newState as unknown) as State;
            if (!this.state.id) {
              this.startShader(defaultShader);
            } else {
              this.loadShaderIfNecessary();
            }
          }
        );
      } catch (error) {
        errorMessage(error);
      }
    });
  }

  private async loadShaderIfNecessary() {
    if (!this.state.id) {
      return;
    }
    try {
      const resp = await fetch(`/_/load/${this.state.id}`, {
        credentials: 'include',
      });
      const json = (await jsonOrThrow(resp)) as ScrapBody;
      this.lastSavedCode = json.Body;
      this.startShader(json.Body);
    } catch (error) {
      errorMessage(error);
      // Return to the default view.
      this.state = Object.assign({}, defaultState);
      this.stateChanged!();
    }
  }

  private startShader(shaderCode: string) {
    // Cancel any pending drawFrames.
    if (this.rafID != RAF_NOT_RUNNING) {
      cancelAnimationFrame(this.rafID);
      this.rafID = RAF_NOT_RUNNING;
    }

    this.startTime = Date.now();
    this.runningCode = shaderCode;
    this.editedCode = shaderCode;
    this.codeMirror!.setValue(shaderCode);

    this.surface?.delete();
    this.surface = this.kit!.MakeCanvasSurface(this.canvasEle!);
    if (!this.surface) {
      errorMessage('Could not make Surface.');
      return;
    }
    // We don't need to call .delete() on the canvas because
    // the parent surface will do that for us.
    this.canvas = this.surface.getCanvas();
    this.canvasKitContext = this.kit!.currentContext();
    this.effect?.delete();
    // TODO(jcgregorio) Once RuntimeEffect.Make() supports an optional callback
    // that reports compilation errors we can supply detailed error messages to
    // the user.
    this.effect = this.kit!.RuntimeEffect.Make(shaderCode);
    if (!this.effect) {
      errorMessage('Could not create RuntimeEffect');
      return;
    }

    this.drawFrame();
  }

  private drawFrame() {
    this.kit!.setCurrentContext(this.canvasKitContext);

    // Build uniforms and pass into makeShader.
    // TODO(jcgregorio) Interrogate the effect for its uniforms.
    const uniforms = [this.playbackPosition];
    const shader = this.effect!.makeShader(uniforms);

    // Draw the shader.
    this.canvas!.clear(this.kit!.BLACK);
    this.paint!.setShader(shader);
    const rect = this.kit!.XYWHRect(0, 0, this.width, this.height);
    this.canvas!.drawRect(rect, this.paint!);
    this.surface!.flush();

    this.rafID = requestAnimationFrame(() => {
      this.drawFrame();
    });
  }

  private async runClick() {
    this.startShader(this.editedCode);
  }

  private async saveClick() {
    const body: ScrapBody = {
      Body: this.editedCode,
      Type: 'sksl',
    };
    try {
      // POST the JSON to /_/upload
      const resp = await fetch('/_/save/', {
        credentials: 'include',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
        method: 'POST',
      });
      const json = (await jsonOrThrow(resp)) as ScrapID;

      this.state.id = json.Hash;
      this.lastSavedCode = this.editedCode;
      this.stateChanged!();
      this._render();
    } catch (error) {
      errorMessage(`${error}`);
    }
  }

  private codeChange() {
    this.editedCode = this.codeMirror!.getValue();
    this._render();
  }

  /** The position in [0, 1] where we are in the playback.
   *
   * The value returned depends on Date.now() and this.startTime.
   */
  get playbackPosition() {
    const elapsedTime = Date.now() - this.startTime;
    let playbackPosition = elapsedTime / this.duration;
    if (playbackPosition > 1) {
      // Make sure we hit the end frame, but set us up to start at the beginning
      // on the next frame.
      playbackPosition = 1;
      this.startTime = Date.now();
    }
    return playbackPosition;
  }
}

define('shaders-app-sk', ShadersAppSk);
