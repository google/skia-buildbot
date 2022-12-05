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
} from '../../wasm_libs/types/canvaskit'; // gazelle:ignore
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/icon/edit-icon-sk';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/app-sk';
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

// A convenience type that represents HTML Elements which are also UniformControls.
interface UniformControlElement extends Element, UniformControl {}

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

/**
 * Each shader example consists of a hash linking it to its corresponding webpages which updates
 * the codemirror and canvas. The image source is the source of the thumbnail image (a jpeg) and
 * the alt is the alternative text
 */
 interface shaderExample {
  hash: string;
  imageName: string;
}
/** An array of shader examples. Each image name must correspond to the thumbnail name */
const exampleShaders: Array<shaderExample> = [
  {
    hash: 'de2a4d7d893a7251eb33129ddf9d76ea517901cec960db116a1bbd7832757c1f',
    imageName: 'blue-neurons',
  },

  {
    hash: 'ed72577c437c036447372e4c873462fc1bbfc0cb5e9fb0630ab1c07368a0db48',
    imageName: 'kaleidoscope',
  },

  {
    hash: 'f9be5248170044ea1b69ee78456eec4be98d3e71a6c61fd0138f6018abda2ac3',
    imageName: 'blue-clouds',
  },

  {
    hash: '2bee4488820c3253cd8861e85ce3cab86f482cfddd79cfd240591bf64f7bcc38',
    imageName: 'fibonacci-sphere',
  },

  {
    hash: '7cd08fc6b1b23121529c62e31e00b4bd6b49deba9a3904fd01fda3dc5c590050',
    imageName: 'mandelbrot',
  },

  {
    hash: '23a360c975c3cb195c89ccdf65ec549e279ce8a959643b447e69cb70614a6eca',
    imageName: 'smoke',
  },

  {
    hash: '80c3854680c3e99d71fbe24d86185d5bb20cb047305242f9ecb5aff0f102cf73',
    imageName: 'snowfall',
  },

  {
    hash: 'e0ec9ef204763445036d8a157b1b5c8929829c3e1ee0a265ed984aeddc8929e2',
    imageName: 'starfield',
  },

  {
    hash: 'e3c8c172e50a69196b2f7712c307ae7099931c3addfc21075ef4ab6aeed11f71',
    imageName: 'switch-color',
  }];

/**
 * A collection of thumbnail snippets that redirect to different shader examples when clicked on.
 * Additional examples can be added in example shaders with corresponding webpage hash and name
 * which in turn links to its thumbnail.
 * @returns the gallery template result
 */
const exampleShadersGalleryTemplate = () => html`
    <div class="gallery-container">
      <div class="thumbnails"></div>
      <div class="scrollbar">
        <div class="scrollbar-thumb"></div>
      </div>
      <div class="slides">
        ${generateExampleShadersHTML()}
      </div>
    </div>
  `;

/**
 * Iterates through example shaders and adds it to an ordered list
 */
const generateExampleShadersHTML = () => html`
  <ol class="slides">
    ${exampleShaders.map((i) => shaderEntry(i))}
  </ol>`;

/**
 * Formats each shader entry attaching the link, image source, and alternative text
 * @param i the shader example
 * @returns formated shader exanple entry div
 */
const shaderEntry = (i: shaderExample) => html`
  <li class="thumbnails">
    <a href=${`https://shaders.skia.org/?id=${i.hash}`}>
      <div>
        <img src=${cdnImage(i)} alt=${`Clickable thumbnail of ${i.imageName} shader example`}>
      </div>
    </a>
  </li>`;

/**
 * Formats the shader example thumbnail url
 * @param i a shader example
 * @returns the corrrect url for a shader example
 */
const cdnImage = (i: shaderExample) => `https://storage.googleapis.com/skia-world-readable/example-shaders/${i.imageName}-thumbnail.jpeg`;

export class ShadersAppSk extends ElementSk {
  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private traceCoordX: number = DEFAULT_SIZE / 2;

  private traceCoordY: number = DEFAULT_SIZE / 2;

  private traceCoordClickMs: number = 0;

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

  // TODO (anjulij): add this back to example shaders
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
  <app-sk>
    <header>
      <a href="/">SkSL Shaders</a>
      <span>
        <a
          id="githash"
          href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
        >
          ${SKIA_VERSION.slice(0, 7)}
        </a>
        <theme-chooser-sk class="theme-chooser"></theme-chooser-sk>
      </span>
    </header>
    <main>
      <div>
        <div class="example-gallery-and-canvas-wrapper">
          <div>
            ${exampleShadersGalleryTemplate()}
          </div>
        <canvas
          id="player"
          width=${ele.width}
          height=${ele.height}
        >
          Your browser does not support the canvas tag.
        </canvas>
        </div>
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
  </app-sk>
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

    // Listen for clicks on the canvas, to update the debug trace coordinate.
    this.canvasEle!.addEventListener('click', (e: MouseEvent) => {
      this.traceCoordX = Math.floor(e.offsetX);
      this.traceCoordY = Math.floor(e.offsetY);
      this.traceCoordClickMs = Date.now();
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
        doNotWait(errorMessage(error as Error, 0));
      }
    });
  }

  private dimensionsChanged(e: Event) {
    const newDims = (e as CustomEvent<DimensionsChangedEventDetail>).detail;
    this.width = newDims.width;
    this.height = newDims.height;
    this.traceCoordX = Math.floor(this.width / 2);
    this.traceCoordY = Math.floor(this.height / 2);
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
      doNotWait(errorMessage(error as Error, 0));
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

  /** Query the uniforms from the controls defined by the user. These uniforms will be packed into
   *  the beginning of the uniform buffer and be the same for all shaders. By being the same, this
   *  will allow a parent shader to be in sync with its children shaders.
   */
  private getPredefinedUniformValuesFromControls(): number[] {
    if (!this.currentNode) {
      return [];
    }
    const uniforms: number[] = new Array(this.currentNode.numPredefinedUniformValues).fill(0);
    $<UniformControlElement>('#uniformControls > *')
      .slice(0, this.numPredefinedUniformControls) // stop after the predefined controls
      .forEach((control: UniformControl) => {
        control.applyUniformValues(uniforms);
      });
    return uniforms;
  }

  /** Query the uniforms from the controls defined by the user. These uniforms will be packed
   *  into memory after the predefined uniforms (e.g. time, resolution).
   */
  private getUserUniformValuesFromControls(): number[] {
    if (!this.currentNode) {
      return [];
    }
    // Make a full array because the application of uniform values are implemented to write to
    // specific indexes and we need those later indexes to exist.
    const uniforms: number[] = new Array(this.currentNode.getUniformFloatCount()).fill(0);
    $<UniformControlElement>('#uniformControls > *')
      .slice(this.numPredefinedUniformControls) // start after the predefined controls
      .forEach((control: UniformControl) => {
        control.applyUniformValues(uniforms);
      });
    // Slice off the uniform values belonging to indices for the predefined uniforms. These
    // should all be zero anyway.
    return uniforms.slice(this.currentNode.numPredefinedUniformValues);
  }

  /** Populate the control values from the uniforms. */
  private setUniformValuesToControls(): void {
    const predefinedUniformValues = new Array(this.currentNode!.numPredefinedUniformValues).fill(0);
    const uniforms = predefinedUniformValues.concat(this.currentNode!.currentUserUniformValues);
    $<UniformControlElement>('#uniformControls > *').forEach((control: UniformControl) => {
      control.restoreUniformValues(uniforms);
    });
    this.findAllUniformControlsThatNeedRAF();
  }

  private findAllUniformControlsThatNeedRAF(): void {
    this.uniformControlsNeedingRAF = [];
    $<UniformControlElement>('#uniformControls > *').forEach((control: UniformControl) => {
      if (control.needsRAF()) {
        this.uniformControlsNeedingRAF.push(control);
      }
    });
  }

  private uniformControlsChange(): void {
    this.currentNode!.currentUserUniformValues = this.getUserUniformValuesFromControls();
    this._render();
  }

  private drawFrame(): void {
    const shader = this.currentNode!.getShader(this.getPredefinedUniformValuesFromControls());
    if (!shader || !this.kit) {
      return;
    }

    // Allow uniform controls to update, such as uniform-timer-sk.
    this.uniformControlsNeedingRAF.forEach((element) => {
      element.onRAF();
    });

    // Draw the shader.
    this.canvas!.clear(this.kit.BLACK);
    this.paint!.setShader(shader);
    const rect = this.kit.XYWHRect(0, 0, this.width, this.height);
    this.canvas!.drawRect(rect, this.paint!);

    // If the user recently clicked on the canvas to set the trace coordinate, circle it.
    // The circle pulses three times; each pulse lasts 200ms.
    const blinkMs: number = Date.now() - this.traceCoordClickMs;
    if (blinkMs < 600) {
      const phase = ((blinkMs % 200) / 200); // increases from 0 to 1 over each pulse
      const paint: Paint = new this.kit.Paint();
      paint.setAntiAlias(true);
      paint.setStyle(this.kit.PaintStyle.Stroke);
      paint.setStrokeWidth(2.0);
      const opacity: number = 1.0 - phase;
      paint.setColor(this.kit.Color4f(1, 0, 0, opacity));
      const size: number = 10 + (phase * 5);
      const ovalFrame = this.kit.XYWHRect(this.traceCoordX - size, this.traceCoordY - size,
        2 * size, 2 * size);
      this.canvas!.drawOval(ovalFrame, paint);
      paint.delete();
    }

    this.surface!.flush();
    shader.delete();

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
      const traced = this.kit.RuntimeEffect.MakeTraced(shader, this.traceCoordX, this.traceCoordY);
      paint.setShader(traced.shader);
      // Clip to a tight rectangle around the trace coordinate to reduce draw time.
      const tightClip = this.kit.XYWHRect(this.traceCoordX - 2, this.traceCoordY - 2, 5, 5);
      canvas.clipRect(tightClip, this.kit.ClipOp.Intersect, /* doAntiAlias= */ false);
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
    shader.delete();
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
      doNotWait(errorMessage(error as Error, 0));
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
      doNotWait(errorMessage(error as Error));
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
