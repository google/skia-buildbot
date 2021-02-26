/**
 * @module modules/shaders-app-sk
 * @description <h2><code>shaders-app-sk</code></h2>
 *
 */
import { $, $$ } from 'common-sk/modules/dom';
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import CodeMirror from 'codemirror';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import type {
  CanvasKit,
  Surface,
  Canvas,
  Paint,
} from '../../build/canvaskit/canvaskit.js';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/icon/edit-icon-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { SKIA_VERSION } from '../../build/version';
import { ElementSk } from '../../../infra-sk/modules/ElementSk/ElementSk';
import '../../../infra-sk/modules/uniform-fps-sk';
import '../../../infra-sk/modules/uniform-time-sk';
import '../../../infra-sk/modules/uniform-generic-sk';
import '../../../infra-sk/modules/uniform-dimensions-sk';
import '../../../infra-sk/modules/uniform-slider-sk';
import '../../../infra-sk/modules/uniform-mouse-sk';
import '../../../infra-sk/modules/uniform-color-sk';
import '../../../infra-sk/modules/uniform-imageresolution-sk';
import { UniformControl } from '../../../infra-sk/modules/uniform/uniform';
import { DimensionsChangedEventDetail } from '../../../infra-sk/modules/uniform-dimensions-sk/uniform-dimensions-sk';
import {
  defaultShader, numPredefinedUniformLines, predefinedUniforms, ShaderNode,
} from '../shadernode';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL((document!.currentScript as HTMLScriptElement).src).origin;
const kitReady = CanvasKitInit({
  locateFile: (file: any) => `${scriptOrigin}/dist/${file}`,
});

const DEFAULT_SIZE = 512;

type stateChangedCallback = ()=> void;

// State represents data reflected to/from the URL.
interface State {
  id: string;
}

const defaultState: State = {
  id: '@default',
};

// CodeMirror likes mode definitions as maps to bools, but a string of space
// separated words is easier to edit, so we convert between the two format.
function words(str: string): {[key: string]: boolean} {
  const obj: any = {};
  str.split(/\s+/).forEach((word) => {
    if (!word) {
      return;
    }
    obj[word] = true;
  });
  return obj;
}

// See the design doc for the list of keywords. http://go/shaders.skia.org.
const keywords = `const attribute uniform varying break continue
  discard return for while do if else struct in out inout uniform layout`;
const blockKeywords = 'case do else for if switch while struct enum union';
const defKeywords = 'struct enum union';
const builtins = `radians degrees
  sin cos tan asin acos atan
  pow exp log exp2 log2
  sqrt inversesqrt
  abs sign floor ceil fract mod
  min max clamp saturate
  mix step smoothstep
  length distance dot cross normalize
  faceforward reflect refract
  matrixCompMult inverse
  lessThan lessThanEqual greaterThan greaterThanEqual equal notEqual
  any all not
  sample unpremul `;

const types = `int long char short double float unsigned
  signed void bool float float2 float3 float4
  float2x2 float3x3 float4x4
  half half2 half3 half4
  half2x2 half3x3 half4x4
  bool bool2 bool3 bool4
  int int2 int3 int4
  fragmentProcessor shader
  vec2 vec3 vec4
  ivec2 ivec3 ivec4
  bvec2 bvec3 bvec4
  mat2 mat3 mat4`;

// Define a new mode and mime-type for SkSL shaders. We follow the shader naming
// covention found in CodeMirror.
CodeMirror.defineMIME('x-shader/x-sksl', {
  name: 'clike',
  keywords: words(keywords),
  types: words(types),
  builtin: words(builtins),
  blockKeywords: words(blockKeywords),
  defKeywords: words(defKeywords),
  typeFirstDefinitions: true,
  atoms: words('sk_FragCoord true false'),
  modeProps: { fold: ['brace', 'include'] },
});


// requestAnimationFrame id if requestAnimationFrame is not running.
const RAF_NOT_RUNNING = -1;

export class ShadersAppSk extends ElementSk {
  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private codeMirror: CodeMirror.Editor | null = null;

  private canvasEle: HTMLCanvasElement | null = null;

  private kit: CanvasKit | null = null;

  private canvasKitContext: number = -1;

  private surface: Surface | null = null;

  private canvas: Canvas | null = null;

  private paint: Paint | null = null;

  private shaderNode: ShaderNode | null = null;

  // Records the lines that have been marked as having errors. We keep these
  // around so we can clear the error annotations efficiently.
  private compileErrorLines: CodeMirror.TextMarker[] = [];

  private state: State = defaultState;

  // The requestAnimationFrame id if we are running, otherwise we are not running.
  private rafID: number = RAF_NOT_RUNNING;

  // stateReflector update function.
  private stateChanged: stateChangedCallback | null = null;

  private uniformControlsNeedingRAF: UniformControl[] = [];

  /**
   * Calculated when we render, it count the number of controls that are for
   * predefined uniforms, as opposed to user uniform controls.
   */
  private numPredefinedUniformControls: number = 0;

  constructor() {
    super(ShadersAppSk.template);
  }

  private static uniformControls = (ele: ShadersAppSk): TemplateResult[] => {
    const ret: TemplateResult[] = [
      html`<uniform-fps-sk></uniform-fps-sk>`, // Always start with the fps control.
    ];
    ele.numPredefinedUniformControls = 1;
    const node = ele.shaderNode;
    if (!node) {
      return ret;
    }
    for (let i = 0; i < node.getUniformCount(); i++) {
      const uniform = node.getUniform(i);
      if (!uniform.name.startsWith('i')) {
        continue;
      }
      switch (uniform.name) {
        case 'iTime':
          ele.numPredefinedUniformControls++;
          ret.push(html`<uniform-time-sk .uniform=${uniform}></uniform-time-sk>`);
          break;
        case 'iMouse':
          ele.numPredefinedUniformControls++;
          ret.push(html`<uniform-mouse-sk .uniform=${uniform} .elementToMonitor=${ele.canvasEle}></uniform-mouse-sk>`);
          break;
        case 'iResolution':
          ele.numPredefinedUniformControls++;
          ret.push(html`
            <uniform-dimensions-sk
              .uniform=${uniform}
              @dimensions-changed=${ele.dimensionsChanged}
            ></uniform-dimensions-sk>`);
          break;
        case 'iImageResolution':
          // No-op. This is no longer handled via uniform control, the
          // dimensions are handed directly into the ShaderNode from the image
          // measurements.
          break;
        default:
          if (uniform.name.toLowerCase().indexOf('color') !== -1) {
            ret.push(html`<uniform-color-sk .uniform=${uniform}></uniform-color-sk>`);
          } else if (uniform.rows === 1 && uniform.columns === 1) {
            ret.push(html`<uniform-slider-sk .uniform=${uniform}></uniform-slider-sk>`);
          } else {
            ret.push(html`<uniform-generic-sk .uniform=${uniform}></uniform-generic-sk>`);
          }
          break;
      }
    }
    return ret;
  }

  private static template = (ele: ShadersAppSk) => html`
    <header>
      <h2><a href="/">SkSL Shaders</a></h2>
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
      <div>
        <p id=examples @click=${ele.fastLoad}>
        Examples:
          <a href="/?id=@inputs">Uniforms</a>
          <a href="/?id=@iResolution">iResolution</a>
          <a href="/?id=@iTime">iTime</a>
          <a href="/?id=@iMouse">iMouse</a>
          <a href="/?id=@iImage">iImage</a>
        </p>
        <canvas
          id="player"
          width=${ele.width}
          height=${ele.height}
        >
          Your browser does not support the canvas tag.
        </canvas>
      </div>
      <div>
        <details id=shaderinputs>
          <summary>Shader Inputs</summary>
          <textarea rows=${numPredefinedUniformLines} cols=75 readonly id="predefinedShaderInputs">${predefinedUniforms}</textarea>
          <div id=imageSources>
            <figure>
              ${ele.shaderNode?.inputImageElement}
              <figcaption>iImage1</figcaption>
            </figure>
            <details id=image_edit>
              <summary><edit-icon-sk></edit-icon-sk></summary>
              <div id=image_edit_dialog>
                <label for=image_url>
                  Change the URL used for the source image.
                </label>
                <div>
                  <input type=url id=image_url placeholder="URL of image to use." .value="${ele.shaderNode?.getSafeImageURL() || ''}">
                  <button @click=${ele.imageURLChanged}>Use</button>
                </div>
                <label for=image_upload>
                  Or upload an image to <em>temporarily</em> try as a source for the shader. Uploaded images are not saved.
                </label>
                <div>
                  <input @change=${ele.imageUploaded} type=file id=image_upload accept="image/*">
                </div>
              </div>
            </details>
          </div>
        </details>
        <div id="codeEditor"></div>
        <div ?hidden=${!ele.shaderNode?.compileErrorMessage} id="compileErrors">
          <h3>Errors</h3>
          <pre>${ele.shaderNode?.compileErrorMessage}</pre>
        </div>
      </div>
      <div id=shaderControls>
        <div id=uniformControls @input=${ele.uniformControlsChange} @change=${ele.uniformControlsChange}>
          ${ShadersAppSk.uniformControls(ele)}
        </div>
        <button
          ?hidden=${!ele.shaderNode?.needsCompile()}
          @click=${ele.runClick}
          class=action
        >
          Run
        </button>
        <button
          ?hidden=${!ele.shaderNode?.needsSave()}
          @click=${ele.saveClick}
          class=action
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
  private static themeFromCurrentMode = () => (isDarkMode() ? 'ambiance' : 'base16-light');

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.canvasEle = $$<HTMLCanvasElement>('#player', this);
    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'x-shader/x-sksl',
      theme: ShadersAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
    });
    this.codeMirror.on('change', () => this.codeChange());

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', ShadersAppSk.themeFromCurrentMode());
    });

    // Continue the setup once CanvasKit WASM has loaded.
    kitReady.then(async (ck: CanvasKit) => {
      this.kit = ck;
      this.paint = new this.kit.Paint();
      try {
        this.stateChanged = stateReflector(
          /* getState */ () => (this.state as unknown) as HintableObject,
          /* setState */ (newState: HintableObject) => {
            this.state = (newState as unknown) as State;
            this.shaderNode = new ShaderNode(this.kit!);
            if (!this.state.id) {
              this.run();
            } else {
              this.loadShaderIfNecessary();
            }
          },
        );
      } catch (error) {
        errorMessage(error, 0);
      }
    });
  }

  private dimensionsChanged(e: Event) {
    const newDims = (e as CustomEvent<DimensionsChangedEventDetail>).detail;
    this.width = newDims.width;
    this.height = newDims.height;
    this.run();
  }

  private monitorIfDevicePixelRatioChanges() {
    // Use matchMedia to detect if the screen resolution changes from the current value.
    // See https://developer.mozilla.org/en-US/docs/Web/API/Window/devicePixelRatio#monitoring_screen_resolution_or_zoom_level_changes
    const mqString = `(resolution: ${window.devicePixelRatio}dppx)`;
    matchMedia(mqString).addEventListener('change', () => this.run());
  }

  private async loadShaderIfNecessary() {
    if (!this.state.id) {
      return;
    }
    try {
      await this.shaderNode!.loadScrap(this.state.id, () => {
        // Re-render once the input image has loaded.
        this._render();
      });
      this._render();

      const predefinedUniformValues = new Array(this.shaderNode!.numPredefinedUniformValues).fill(0);
      this.setUniformValuesToControls(predefinedUniformValues.concat(this.shaderNode!.currentUserUniformValues));
      this.findAllUniformControlsThatNeedRAF();

      this.run();
    } catch (error) {
      errorMessage(error, 0);
      // Return to the default view.
      this.state = Object.assign({}, defaultState);
      this.stateChanged!();
    }
  }

  private run() {
    this.monitorIfDevicePixelRatioChanges();
    // Cancel any pending drawFrames.
    if (this.rafID !== RAF_NOT_RUNNING) {
      cancelAnimationFrame(this.rafID);
      this.rafID = RAF_NOT_RUNNING;
    }

    this.codeMirror!.setValue(this.shaderNode?.shaderCode || defaultShader);

    // eslint-disable-next-line no-unused-expressions
    this.surface?.delete();
    this.surface = this.kit!.MakeCanvasSurface(this.canvasEle!);
    if (!this.surface) {
      errorMessage('Could not make Surface.', 0);
      return;
    }
    // We don't need to call .delete() on the canvas because
    // the parent surface will do that for us.
    this.canvas = this.surface.getCanvas();
    this.canvasKitContext = this.kit!.currentContext();
    this.clearAllEditorErrorAnnotations();

    this.shaderNode!.compile();

    // Set CodeMirror errors if the run failed.
    this.shaderNode!.compileErrorLineNumbers.forEach((lineNumber: number) => {
      this.setEditorErrorLineAnnotation(lineNumber);
    });

    // Render so the uniform controls get displayed.
    this._render();
    this.drawFrame();
  }

  private clearAllEditorErrorAnnotations(): void{
    // eslint-disable-next-line no-unused-expressions
    this.compileErrorLines?.forEach((textMarker) => {
      textMarker.clear();
    });
  }

  private setEditorErrorLineAnnotation(lineNumber: number): void {
    // Set the class of that line to 'cm-error'.
    this.compileErrorLines.push(this.codeMirror!.markText(
      { line: lineNumber - 1, ch: 0 },
      { line: lineNumber - 1, ch: 200 }, // Some large number for the character offset.
      {
        className: 'cm-error', // See the base16-dark.css file in CodeMirror for the class name.
      },
    ));
  }

  /** Populate the uniforms values from the controls. */
  private getUserUniformValuesFromControls(): number[] {
    const uniforms: number[] = new Array(this.shaderNode!.getUniformFloatCount()).fill(0);
    $('#uniformControls > *').slice(this.numPredefinedUniformControls).forEach((control) => {
      (control as unknown as UniformControl).applyUniformValues(uniforms);
    });
    return uniforms.slice(this.shaderNode?.numPredefinedUniformValues || 0);
  }

  private getPredefinedUniformValuesFromControls(): number[] {
    const uniforms: number[] = new Array(this.shaderNode!.getUniformFloatCount()).fill(0);
    $('#uniformControls > *').slice(0, this.numPredefinedUniformControls).forEach((control) => {
      (control as unknown as UniformControl).applyUniformValues(uniforms);
    });
    return uniforms;
  }

  /** Populate the control values from the uniforms. */
  private setUniformValuesToControls(uniforms: number[]): void {
    $('#uniformControls > *').forEach((control) => {
      (control as unknown as UniformControl).restoreUniformValues(uniforms);
    });
  }

  private findAllUniformControlsThatNeedRAF(): void {
    this.uniformControlsNeedingRAF = [];
    $('#uniformControls > *').forEach((control) => {
      const uniformControl = (control as unknown as UniformControl);
      if (uniformControl.needsRAF()) {
        this.uniformControlsNeedingRAF.push(uniformControl);
      }
    });
  }

  private uniformControlsChange() {
    this.shaderNode!.currentUserUniformValues = this.getUserUniformValuesFromControls();
    this._render();
  }

  private drawFrame() {
    this.kit!.setCurrentContext(this.canvasKitContext);
    const shader = this.shaderNode!.getShader(this.getPredefinedUniformValuesFromControls());
    if (!shader) {
      return;
    }

    // Allow uniform controls to update, such as uniform-timer-sk.
    this.uniformControlsNeedingRAF.forEach((element) => {
      element.onRAF();
    });

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
    this.run();
    this.saveClick();
  }

  private async saveClick() {
    try {
      this.state.id = await this.shaderNode!.saveScrap();
      this.stateChanged!();
      this._render();
    } catch (error) {
      errorMessage(`${error}`, 0);
    }
  }

  private imageUploaded(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) {
      return;
    }
    const file = input.files.item(0)!;
    this.setCurrentImageURL(URL.createObjectURL(file));
  }

  private imageURLChanged(): void {
    const input = $$<HTMLInputElement>('#image_url', this)!;
    if (!input.value) {
      return;
    }
    this.setCurrentImageURL(input.value);
  }

  private codeChange() {
    this.shaderNode!.shaderCode = this.codeMirror!.getValue();
    this._render();
  }

  /**
   * Load example by changing state rather than actually following the links.
   */
  private fastLoad(e: Event): void {
    const ele = (e.target as HTMLLinkElement);
    if (ele.tagName !== 'A') {
      return;
    }
    e.preventDefault();
    // When switching shaders clear the file upload.
    $$<HTMLInputElement>('#image_upload')!.value = '';
    const id = new URL(ele.href).searchParams.get('id') || '';
    this.state.id = id;
    this.stateChanged!();
    this.loadShaderIfNecessary();
  }

  private setCurrentImageURL(url: string): void {
    const oldURL = this.shaderNode!.getCurrentImageURL();

    // Release unused memory.
    if (oldURL.startsWith('blob:')) {
      URL.revokeObjectURL(oldURL);
    }

    this.shaderNode!.setCurrentImageURL(url, () => this._render());
  }
}

define('shaders-app-sk', ShadersAppSk);
