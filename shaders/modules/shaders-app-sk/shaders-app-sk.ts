/**
 * @module modules/shaders-app-sk
 * @description <h2><code>shaders-app-sk</code></h2>
 *
 */
import { $, $$ } from 'common-sk/modules/dom';
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { unsafeHTML } from 'lit-html/directives/unsafe-html';
import { errorMessage } from 'elements-sk/errorMessage';
import CodeMirror from 'codemirror';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import type {
  Canvas,
  CanvasKit,
  CanvasKitInit as CKInit,
  Paint,
  Surface,
} from '../../wasm_libs/types/canvaskit';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/icon/edit-icon-sk';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
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
  defaultScrapBody,
  defaultShader, numPredefinedUniformLines, predefinedUniforms, ShaderNode,
} from '../shadernode';
import { EditChildShaderSk } from '../edit-child-shader-sk/edit-child-shader-sk';
import '../edit-child-shader-sk';
import * as SkSLConstants from '../sksl-constants/sksl-constants';

// It is assumed that canvaskit.js has been loaded and this symbol is available globally.
declare const CanvasKitInit: typeof CKInit;

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL((document!.currentScript as HTMLScriptElement).src).origin;
const kitReady = CanvasKitInit({
  locateFile: (file: string) => `${scriptOrigin}/dist/${file}`,
});

const DEFAULT_SIZE = 512;

type stateChangedCallback = ()=> void;

// This works around a TS lint rule included in Bazel rules which requires promises be awaited
// on. We do not necessarily want to await on errorMessage, especially when we tell the error
// message to be up indefinitely.
// eslint-disable-next-line @typescript-eslint/no-empty-function,@typescript-eslint/no-unused-vars
function doNotWait(_: Promise<unknown>) {}

// State represents data reflected to/from the URL.
interface State {
  id: string;
}

const defaultState: State = {
  id: '@default',
};

// Define a new mode and mime-type for SkSL shaders. We follow the shader naming
// covention found in CodeMirror.
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

/** requestAnimationFrame id if requestAnimationFrame is not running. */
const RAF_NOT_RUNNING = -1;

export class ShadersAppSk extends ElementSk {
  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private codeMirror: CodeMirror.Editor | null = null;

  private canvasEle: HTMLCanvasElement | null = null;

  private kit: CanvasKit | null = null;

  private surface: Surface | null = null;

  private canvas: Canvas | null = null;

  private paint: Paint | null = null;

  private rootShaderNode: ShaderNode | null = null;

  private currentNode: ShaderNode | null = null;

  /**
   * Records the lines that have been marked as having errors. We keep these
   * around so we can clear the error annotations efficiently.
   */
  private compileErrorLines: CodeMirror.TextMarker[] = [];

  private state: State = defaultState;

  /** The requestAnimationFrame id if we are running, otherwise we are not running. */
  private rafID: number = RAF_NOT_RUNNING;

  /** stateReflector update function. */
  private stateChanged: stateChangedCallback | null = null;

  private uniformControlsNeedingRAF: UniformControl[] = [];

  /**
   * Calculated when we render, it count the number of controls that are for
   * predefined uniforms, as opposed to user uniform controls.
   */
  private numPredefinedUniformControls: number = 0;

  private editChildShaderControl: EditChildShaderSk | null = null;

  constructor() {
    super(ShadersAppSk.template);
  }

  private static deleteButton = (ele: ShadersAppSk, parentNode: ShaderNode | null, node: ShaderNode, index: number): TemplateResult => {
    if (ele.rootShaderNode === node || parentNode === null) {
      return html``;
    }
    return html`
      <button
        class=deleteButton
        title="Delete child shader."
        @click=${(e: Event) => ele.removeChildShader(e, parentNode, index)}>
        <delete-icon-sk></delete-icon-sk>
      </button>
    `;
  }

  private static editButton = (ele: ShadersAppSk, parentNode: ShaderNode | null, node: ShaderNode, index: number): TemplateResult => {
    if (ele.rootShaderNode === node || parentNode === null) {
      return html``;
    }
    return html`
      <button
        class=editButton
        title="Edit child shader uniform name."
        @click=${(e: Event) => ele.editChildShader(e, parentNode, index)}>
        <edit-icon-sk></edit-icon-sk>
      </button>
    `;
  }

  private static displayShaderTreeImpl = (ele: ShadersAppSk, parentNode: ShaderNode | null, node: ShaderNode, depth: number = 0, name: string = '/', childIndex: number = 0): TemplateResult[] => {
    let ret: TemplateResult[] = [];
    // Prepend some fixed width spaces based on the depth so we get a nested
    // directory type of layout. See https://en.wikipedia.org/wiki/Figure_space.
    const prefix = new Array(depth).fill('&numsp;&numsp;').join('');
    ret.push(html`
      <p
        class="childShader"
        @click=${() => ele.childShaderClick(node)}>
          <span>
            ${unsafeHTML(prefix)}
            <span class=linkish>${name}</span>
            ${(ele.rootShaderNode!.children.length > 0 && ele.currentNode === node) ? '*' : ''}
          </span>
          <span>
            ${ShadersAppSk.deleteButton(ele, parentNode, node, childIndex)}
            ${ShadersAppSk.editButton(ele, parentNode, node, childIndex)}
            <button
              class=addButton
              title="Append a new child shader."
              @click=${(e: Event) => ele.appendChildShader(e, node)}>
                <add-icon-sk></add-icon-sk>
            </button>
          </span>
      </p>`);
    node.children.forEach((childNode, index) => {
      ret = ret.concat(ShadersAppSk.displayShaderTreeImpl(ele, node, childNode, depth + 1, node.getChildShaderUniformName(index), index));
    });
    return ret;
  }

  private static displayShaderTree = (ele: ShadersAppSk): TemplateResult[] => {
    if (!ele.rootShaderNode) {
      return [
        html``,
      ];
    }
    return ShadersAppSk.displayShaderTreeImpl(ele, null, ele.rootShaderNode);
  }

  private static uniformControls = (ele: ShadersAppSk): TemplateResult[] => {
    const ret: TemplateResult[] = [
      html`<uniform-fps-sk></uniform-fps-sk>`, // Always start with the fps control.
    ];
    ele.numPredefinedUniformControls = 1;
    const node = ele.currentNode;
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
        <theme-chooser-sk></theme-chooser-sk>
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
        <div>
          ${ShadersAppSk.displayShaderTree(ele)}
        </div>
      </div>
      <div>
        <details id=shaderinputs>
          <summary>Shader Inputs</summary>
          <textarea rows=${numPredefinedUniformLines} cols=75 readonly id="predefinedShaderInputs">${predefinedUniforms}</textarea>
          <div id=imageSources>
            <figure>
              ${ele.currentNode?.inputImageElement}
              <figcaption>iImage1</figcaption>
            </figure>
            <details id=image_edit>
              <summary><edit-icon-sk></edit-icon-sk></summary>
              <div id=image_edit_dialog>
                <label for=image_url>
                  Change the URL used for the source image.
                </label>
                <div>
                  <input type=url id=image_url placeholder="URL of image to use." .value="${ele.currentNode?.getSafeImageURL() || ''}">
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
        <textarea style="display: ${ele.currentNode?.children.length ? 'block' : 'none'}" rows=${ele.currentNode?.children.length || 0} cols=75>${ele.currentNode?.getChildShaderUniforms() || ''}</textarea>
        <div id="codeEditor"></div>
        <div ?hidden=${!ele.currentNode?.compileErrorMessage} id="compileErrors">
          <h3>Errors</h3>
          <pre>${ele.currentNode?.compileErrorMessage}</pre>
        </div>
      </div>
      <div id=shaderControls>
        <div id=uniformControls @input=${ele.uniformControlsChange} @change=${ele.uniformControlsChange}>
          ${ShadersAppSk.uniformControls(ele)}
        </div>
        <button
          ?hidden=${!ele.rootShaderNode?.needsCompile()}
          @click=${ele.runClick}
          class=action
        >
          Run
        </button>
        <button
          ?hidden=${!ele.rootShaderNode?.needsSave()}
          @click=${ele.saveClick}
          class=action
        >
          Save
        </button>
        <button
          ?hidden=${ele.currentNode?.compileErrorMessage || ele.rootShaderNode?.needsCompile()}
          @click=${ele.createDebugTrace}
          class=action
        >
          Debug
        </button>
      </div>
    </main>
    <footer>
      <edit-child-shader-sk></edit-child-shader-sk>
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
      scrollbarStyle: 'native',
    });
    this.codeMirror.on('change', () => this.codeChange());
    this.editChildShaderControl = $$('edit-child-shader-sk', this);

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
          /* setState */ async (newState: HintableObject) => {
            this.state = (newState as unknown) as State;
            this.rootShaderNode = new ShaderNode(this.kit!);
            this.currentNode = this.rootShaderNode;
            if (!this.state.id) {
              await this.rootShaderNode.setScrap(defaultScrapBody);
              this.run();
            } else {
              await this.loadShaderIfNecessary();
            }
          },
        );
      } catch (error) {
        doNotWait(errorMessage(error, 0));
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
      await this.rootShaderNode!.loadScrap(this.state.id);
      this._render();
      this.setUniformValuesToControls();
      this.run();
    } catch (error) {
      doNotWait(errorMessage(error, 0));
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

    // TODO(jcgregorio) In the long run maybe store scrollInfo and cursorPos
    // temporarily for each child shader so we restore the right view as the
    // user moves between child shaders?

    // Save the scroll info and the cursor position before updating the code.
    const scrollInfo = this.codeMirror!.getScrollInfo();
    const cursorPos = this.codeMirror!.getCursor();

    // Set code.
    this.codeMirror!.setValue(this.currentNode?.shaderCode || defaultShader);

    // Restore scroll info and cursor position.
    this.codeMirror!.setCursor(cursorPos);

    // Oddly CodeMirror TS doesn't have a Type defined for this shape.
    const scrollPosition = {
      left: scrollInfo.left,
      top: scrollInfo.top,
      right: scrollInfo.left + scrollInfo.width,
      bottom: scrollInfo.top + scrollInfo.height,
    };
    this.codeMirror!.scrollIntoView(scrollPosition);

    // eslint-disable-next-line no-unused-expressions
    this.surface?.delete();
    this.surface = this.kit!.MakeCanvasSurface(this.canvasEle!);
    if (!this.surface) {
      doNotWait(errorMessage('Could not make Surface.', 0));
      return;
    }
    // We don't need to call .delete() on the canvas because
    // the parent surface will do that for us.
    this.canvas = this.surface.getCanvas();
    this.clearAllEditorErrorAnnotations();

    this.rootShaderNode!.compile();

    // Set CodeMirror errors if the run failed.
    this.currentNode!.compileErrorLineNumbers.forEach((lineNumber: number) => {
      this.setEditorErrorLineAnnotation(lineNumber);
    });

    // Render so the uniform controls get displayed.
    this._render();
    this.drawFrame();
  }

  private clearAllEditorErrorAnnotations(): void {
    // eslint-disable-next-line no-unused-expressions
    this.compileErrorLines?.forEach((textMarker) => {
      textMarker.clear();
    });
  }

  private setEditorErrorLineAnnotation(lineNumber: number): void {
    // Set the class of that line to 'cm-error'.
    this.compileErrorLines.push(this.codeMirror!.markText(
      { line: lineNumber - 1, ch: 0 },
      { line: lineNumber - 1, ch: 9999 }, // Some large number for the character offset.
      {
        className: 'cm-error', // See the ambiance.css file in CodeMirror for the class name.
      },
    ));
  }

  /** Populate the uniforms values from the controls. */
  private getUserUniformValuesFromControls(): number[] {
    const uniforms: number[] = new Array(this.currentNode!.getUniformFloatCount()).fill(0);
    $('#uniformControls > *').slice(this.numPredefinedUniformControls).forEach((control) => {
      (control as unknown as UniformControl).applyUniformValues(uniforms);
    });
    return uniforms.slice(this.currentNode?.numPredefinedUniformValues || 0);
  }

  private getPredefinedUniformValuesFromControls(): number[] {
    const uniforms: number[] = new Array(this.currentNode!.getUniformFloatCount()).fill(0);
    $('#uniformControls > *').slice(0, this.numPredefinedUniformControls).forEach((control) => {
      (control as unknown as UniformControl).applyUniformValues(uniforms);
    });
    return uniforms;
  }

  /** Populate the control values from the uniforms. */
  private setUniformValuesToControls(): void {
    const predefinedUniformValues = new Array(this.currentNode!.numPredefinedUniformValues).fill(0);
    const uniforms = predefinedUniformValues.concat(this.currentNode!.currentUserUniformValues);
    $('#uniformControls > *').forEach((control) => {
      (control as unknown as UniformControl).restoreUniformValues(uniforms);
    });
    this.findAllUniformControlsThatNeedRAF();
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

  private uniformControlsChange(): void {
    this.currentNode!.currentUserUniformValues = this.getUserUniformValuesFromControls();
    this._render();
  }

  private drawFrame(): void {
    const shader = this.currentNode!.getShader(this.getPredefinedUniformValuesFromControls());
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

  private createDebugTrace(): void {
    const shader = this.currentNode!.getShader(this.getPredefinedUniformValuesFromControls());
    if (!shader || !this.kit) {
      return;
    }

    // Debug traces require a software surface.
    const surface: Surface = this.kit.MakeSurface(this.width, this.height)!;
    if (surface) {
      const canvas: Canvas = surface.getCanvas();
      const paint: Paint = new this.kit.Paint();
      const traceCoordX: number = this.width / 2;
      const traceCoordY: number = this.height / 2;
      const traced = this.kit.RuntimeEffect.MakeTraced(shader, traceCoordX, traceCoordY);
      paint.setShader(traced.shader);
      // Clip to a tight rectangle around the trace coordinate to reduce draw time.
      const tightClip = this.kit.XYWHRect(traceCoordX - 2, traceCoordY - 2, 5, 5);
      canvas.clipRect(tightClip, this.kit.ClipOp.Intersect, /*doAntiAlias=*/ false);
      const rect = this.kit.XYWHRect(0, 0, this.width, this.height);
      canvas.drawRect(rect, paint);
      const traceJSON: string = traced.debugTrace.writeTrace();

      traced.shader.delete();
      traced.debugTrace.delete();
      paint.delete();
      surface.delete();

      // Write our trace JSON to HTML local storage, where the debugger can see it.
      localStorage.setItem('sksl-debug-trace', traceJSON);

      // Open the debugger in a separate tab, pointing it at our local storage buffer.
      window.open('/debug?local-storage', 'sksl-debug-target');
    }
  }

  private async runClick() {
    this.run();
    await this.saveClick();
  }

  private async saveClick() {
    try {
      this.state.id = await this.rootShaderNode!.saveScrap();
      this.stateChanged!();
      this._render();
    } catch (error) {
      doNotWait(errorMessage(`${error}`, 0));
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
    if (!this.currentNode) {
      return;
    }
    this.currentNode.shaderCode = this.codeMirror!.getValue();
    this._render();
  }

  private async appendChildShader(e: Event, node: ShaderNode) {
    e.stopPropagation();
    try {
      await node.appendNewChildShader();
      this._render();
      await this.runClick();
    } catch (error) {
      doNotWait(errorMessage(error));
    }
  }

  private async removeChildShader(e: Event, parentNode: ShaderNode, index: number) {
    e.stopPropagation();
    // We could write a bunch of complicated logic to track which current shader
    // is selected and restore that correctly on delete, or we can just always
    // shove the focus back to rootShaderNode which will always work, so we do
    // the latter.
    this.childShaderClick(this.rootShaderNode!);

    parentNode.removeChildShader(index);
    this._render();
    await this.runClick();
  }

  private async editChildShader(e: Event, parentNode: ShaderNode, index: number) {
    e.stopPropagation();
    const editedChildShader = await this.editChildShaderControl!.show(parentNode.getChildShader(index));
    if (!editedChildShader) {
      return;
    }
    await parentNode.setChildShaderUniformName(index, editedChildShader.UniformName);
    this._render();
    await this.runClick();
  }

  private childShaderClick(node: ShaderNode) {
    this.currentNode = node;
    this.codeMirror!.setValue(this.currentNode?.shaderCode || defaultShader);
    this._render();
    this.setUniformValuesToControls();
  }

  /**
   * Load example by changing state rather than actually following the links.
   */
  private async fastLoad(e: Event): Promise<void> {
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
    await this.loadShaderIfNecessary();
  }

  private setCurrentImageURL(url: string): void {
    const oldURL = this.currentNode!.getCurrentImageURL();

    // Release unused memory.
    if (oldURL.startsWith('blob:')) {
      URL.revokeObjectURL(oldURL);
    }

    this.currentNode!.setCurrentImageURL(url).then(() => this._render());
  }
}

define('shaders-app-sk', ShadersAppSk);
