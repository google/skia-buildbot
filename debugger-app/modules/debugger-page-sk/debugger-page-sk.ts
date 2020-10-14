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

// Types for the wasm bindings
import { Debugger, DebuggerInitOptions, SkpDebugPlayer, SkSurface } from '../debugger';

// other modules from this application
import '../filter-sk';
import '../play-sk';
import '../debug-view-sk';
import '../matrix-clip-controls-sk';

// TODO(nifong): find a way to move this declaration outside this file
declare function DebuggerInit(opts: DebuggerInitOptions): Promise<Debugger>;

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
      <a class="version-link" href="https://skia.googlesource.com/skia/+show/${ele._skiaVersion}"
         title="The skia commit at which the debugger WASM module was built">
        ${ele._skiaVersionShort}
      </a>
    </header>
    <div id=content>
      <div class="horizontal-flex">
        <label>SKP to open:</label>
        <input type="file" @change=${ele._fileInputChanged}
         ?disabled=${ele._debugger === null} />
        <p>File version: ${ele._fileContext?.version}</p>
      </div>
      <multi-frame-controls-sk></multi-frame-controls-sk>
      <div class="horizontal-flex">
        <div>
          <filter-sk></filter-sk>
          <play-sk></play-sk>
          <commands-sk></commands-sk>
        </div>
        <div id=center>
          <debug-view-sk></debug-view-sk>
        </div>
        <div id=right>
          <matrix-clip-controls-sk></matrix-clip-controls-sk>
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
  private _debugger: Debugger | null = null; // null until the DebuggerInit promise resolves.
  private _surface: SkSurface | null = null; // null until either file loaded or cpu/gpu switch toggled

  // submodules are null until first template render
  private _debugViewSk: DebugViewSk | null = null;

  constructor() {
    super(DebuggerPageSk.template);

    DebuggerInit({
      locateFile: (file: string) => '/dist/'+file,
    }).then((loadedWasmModule) => {
      // Save a reference to the module somewhere we can use it later.
      this._debugger = loadedWasmModule;
      // File input element should now be enabled, so we need to render.
      this._render();
    });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._debugViewSk = this.querySelector<DebugViewSk>('debug-view-sk')!;
  }

  // Called when the filename in the file input element changs
  private _fileInputChanged(e: Event) {
    // Did the change event result in the file-input element specifing a file?
    // (user might have cancelled the dialog)
    const element = e.target as HTMLInputElement;
    if (element.files?.length === 0) {
      return;
    }
    const file = element.files![0];
    // Create a reader and a callback for when the file finishes being read.
    const reader = new FileReader();
    reader.onload = (e: ProgressEvent<FileReader>) => {
      if (e.target) {
        this._openSkpFile(e.target.result as ArrayBuffer);
      }
    };
    reader.readAsArrayBuffer(file);
  }

  // Open an SKP or MSKP file. fileContents is expected to be an arraybuffer
  // with the file's contents
  private _openSkpFile(fileContents: ArrayBuffer) {
    if (!this._debugger) { return; }
    // Create the instance of SkpDebugPlayer and load the file.
    // This function is provided by helper.js in the JS bundled with the wasm module.
    const playerResult = this._debugger.SkpFilePlayer(fileContents);
    if (playerResult.error) {
      errorMessage(`SKP deserialization error: ${playerResult.error}`);
      return;
    }
    this._fileContext = {
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
  private _replaceSurface() {
    if (!(this._debugger && this._debugViewSk)) { return; }

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
    const canvas = this._debugViewSk.resize(width, height);
    // TODO(nifong): get this from toggle switch
    const useWebGL = true;
    // free the wasm memory of the previous surface
    if (this._surface) { this._surface.dispose(); }
    if (useWebGL) {
      this._surface = this._debugger.MakeWebGLCanvasSurface(canvas);
    } else {
      this._surface = this._debugger.MakeSWCanvasSurface(canvas);
    }
  }
};

define('debugger-page-sk', DebuggerPageSk);
