/**
 * @module modules/debugger-page-sk
 * @description The main module of the wasm-based SKP and MSKP debugger.
 *  Holds the loaded wasm module, pointer to the SkSurface in wasm, and SKP file state.
 *  Handles all the interaction and control of the application that does not cleanly fit
 *  within a submodule.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { DebugViewSk } from '../debug-view-sk/debug-view-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk';

// other modules from this application
import '../filter-sk';
import '../play-sk';
import '../debug-view-sk';
import '../matrix-clip-controls-sk';


// TODO(nifong): move types for Debugger bindings to a better place
export interface DebuggerInitOptions {
    locateFile: (path: string) => string;
}
// defined in global scope by debugger.js loaded by main.html
declare function DebuggerInit(opts: DebuggerInitOptions): Promise<Debugger>;
export interface Debugger {
  // defined in wasm-skp-debugger/helper.js
  SkpFilePlayer(ab: ArrayBuffer): SkpFilePlayerResult;
  MakeWebGLCanvasSurface(canvas: HTMLCanvasElement): SkSurface;
  MakeSWCanvasSurface(canvas: HTMLCanvasElement): SkSurface;
}
// An object containing either the successfully loaded file player or an error.
export interface SkpFilePlayerResult {
  readonly error: string;
  readonly player: SkpDebugPlayer;
}
export interface SkpDebugPlayer {
  changeFrame(index: number): void;
  deleteCommand(index: number): void;
  draw(surface: SkSurface): void;
  drawTo(surface: SkSurface, index: number): void;
  fileVersion(): number;
  getBounds(): SkIRect;
  getFrameCount(): number;
  getImageResource(index: number): string;
  getImageCount(): number;
  getImageInfo(index: number): SimpleImageInfo;
  // This returns a built in emscripten binding of a std::vector<DebugLayerManager.LayerSummary>
  // TODO(nifong) make debugger just return json here
  //getLayerSummaries(): string;
  getSize(): number;
  imageUseInfoForFrame(frame: number): string;
  jsonCommandList(surface: SkSurface): string;
  lastCommandInfo(): string;
  loadSkp(ptr: number, len: number): string;
  setClipVizColor(color: Color): void;
  setCommandVisibility(index: number, visible: boolean): void;
  setGpuOpBounds(visible: boolean): void;
  setInspectedLayer(nodeId: number): void;
  setOriginVisible(visible: boolean): void;
  setOverdrawVis(visible: boolean): void;
  setAndroidClipViz(visible: boolean): void;
}
export interface SkSurface {
  dispose(): void;
  flush(): void;
}
export interface SimpleImageInfo {

}
export interface SkIRect {
  fLeft: number;
  fTop: number;
  fRight: number;
  fBottom: number;
}
export interface Color {

}


interface SubModules {
  debuggerView: DebugViewSk;
}
interface FileContext {
  player: SkpDebugPlayer;
  version: number;
}

export class DebuggerPageSk extends ElementSk {
  private static template = (ele: DebuggerPageSk) =>
    html`
    <header>
      <h2>Skia WASM Debugger</h2>
      <a id="version-link" href="https://skia.googlesource.com/skia/+show/${ele._skiaVersion}"
         title="The skia commit at which the debugger WASM module was built">
        ${ele._skiaVersionShort}
      </a>
    </header>
    <div id=content>
      <div class="horizontal-flex">
        <label>SKP to open:</label>
        <input type="file" id="file-input" @change=${ele._fileInputChanged} disabled />
        <p>File version: ${ele._fileContext?.version}</p>
      </div>
      <multi-frame-controls-sk id></multi-frame-controls-sk>
      <div class="horizontal-flex">
        <div>
          <filter-sk id=filter></filter-sk>
          <play-sk id=play></play-sk>
          <commands-sk id=commands></commands-sk>
        </div>
        <div id=center>
          <debug-view-sk id="debug-view"></debug-view-sk>
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

  private _skiaVersion: string = 'a url of some kind';
  private _skiaVersionShort: string = '-1';

  private _fileContext: FileContext | null = null; // null as long as no file loaded.
  private _modules: SubModules | null = null; // null until first template render
  private _debugwasm: Debugger | null = null; // null until the DebuggerInit promise resolves.
  private _surface: SkSurface | null = null; // null until either file loaded or cpu/gpu switch toggled

  constructor() {
    super(DebuggerPageSk.template);

    DebuggerInit({
      locateFile: (file: string) => '/dist/'+file,
    }).then((Debugger) => {
      // Save a reference to the module somewhere we can use it later.
      this._debugwasm = Debugger;
      // Enable the file input element.
      const element = <HTMLInputElement> document.getElementById('file-input');
      element.disabled = false;
    });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    // Now that they exist, find and save references to all submodules.
    this._modules = <SubModules> {
      debuggerView: <DebugViewSk> document.getElementById('debug-view'),
    };
  }

  // Called when the filename in the file input element changs
  _fileInputChanged(e: Event) {
    // Did the change event result in the file-input element specifing a file?
    // (user might have cancelled the dialog)
    const element = <HTMLInputElement> e.target;
    if (!element.files || !element.files[0]) {
      return;
    }
    const file = element.files[0];
    // Create a reader and a callback for when the file finishes being read.
    const reader = new FileReader();
    reader.onload = (e) => {
      // this event is a ProgressEvent, e.target is the FileReader, e.target.result
      // is an ArrayBuffer
      if (e.target) {
        this._openSkpFile(<ArrayBuffer> e.target.result);
      }
    };
    reader.readAsArrayBuffer(file);
  }

  // Open an SKP or MSKP file. fileContents is expected to be an arraybuffer
  // with the file's contents
  _openSkpFile(fileContents: ArrayBuffer) {
    if (!this._debugwasm) { return; }
    // Create the instance of SkpDebugPlayer and load the file.
    // This function is provided by helper.js in the JS bundled with the wasm module.
    const playerResult = this._debugwasm.SkpFilePlayer(fileContents);
    if (playerResult.error) {
      errorMessage(`SKP deserialization error: ${playerResult['error']}`);
      return;
    }
    this._fileContext = <FileContext> {
      player: playerResult.player,
      version: playerResult.player.fileVersion(),
    };
    this._replaceSurface();
    if (!this._surface) {
      errorMessage("Could not create SkSurface, try GPU/CPU toggle.");
      return;
    }
    // TODO(nifong): Draw the rest of the owl

    this._render();

    // TODO(nifong): remove test draw after getting command list online,
    this._fileContext.player.draw(this._surface);
    this._surface.flush();
  }

  // Create a new drawing surface. this is called when
  // * GPU/CPU mode changes
  // * Bounds of the skp change (skp loaded)
  // * (not yet supported) Color mode changes
  _replaceSurface() {
    if (!(this._modules && this._debugwasm)) { return; }

    let width = 400;
    let height = 400;
    if (this._fileContext) {
      // From the loaded SKP, player knows how large its picture is. Resize our canvas to match.
      let bounds = this._fileContext.player.getBounds();
      width = bounds.fRight - bounds.fLeft;
      height = bounds.fBottom - bounds.fTop;
      console.log(`SKP width ${width} height ${height}`);
      // Still ok to proceed if no skp, the toggle still should work before a file is picked.
    }
    const canvas = this._modules.debuggerView.resize(width, height);
    // TODO(nifong): get this from toggle switch
    const useWebGL = true;
    // free the wasm memory of the previous surface
    if (this._surface) { this._surface.dispose(); }
    if (useWebGL) {
      this._surface = this._debugwasm.MakeWebGLCanvasSurface(canvas);
    } else {
      this._surface = this._debugwasm.MakeSWCanvasSurface(canvas);
    }
  }
};

define('debugger-page-sk', DebuggerPageSk);
