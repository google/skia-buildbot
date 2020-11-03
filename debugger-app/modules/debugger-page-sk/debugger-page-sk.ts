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
// Import the checkbox element used in in the template
import 'elements-sk/checkbox-sk';
// import the type of the chckbox element
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { DebugViewSk } from '../debug-view-sk/debug-view-sk';
import {
  CommandsSk, PrefixItem, Command, CommandsSkMovePositionEventDetail
} from '../commands-sk/commands-sk';
import { PlaySk } from '../play-sk/play-sk';
import { ZoomSk, ZoomSkPointEventDetail, Point } from '../zoom-sk/zoom-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk';

// Types for the wasm bindings
import {
  Debugger, DebuggerInitOptions, SkpDebugPlayer, SkSurface, SkpJsonCommandList,
  MatrixClipInfo, Matrix3x3, Matrix4x4
} from '../debugger';

// other modules from this application
import '../commands-sk';
import '../debug-view-sk';
import '../histogram-sk';
import '../zoom-sk';

// TODO(nifong): find a way to move this declaration outside this file
declare function DebuggerInit(opts: DebuggerInitOptions): Promise<Debugger>;

interface FileContext {
  player: SkpDebugPlayer;
  version: number;
};

export interface DebuggerPageSkLightDarkEventDetail {
  mode: string;
}

export class DebuggerPageSk extends ElementSk {
  private static template = (ele: DebuggerPageSk) =>
    html`
    <header>
      <h2>Skia WASM Debugger</h2>
      <a class="version-link"
         href="https://skia.googlesource.com/skia/+show/${ele._skiaVersion}"
         title="The skia commit at which the debugger WASM module was built">
        ${ele._skiaVersionShort}
      </a>
    </header>
    <div id=content>
      <div class="horizontal-flex">
        <label>SKP to open:</label>
        <input type="file" @change=${ele._fileInputChanged}
         ?disabled=${ele._debugger === null} />
        <a href="https://skia.org/dev/tools/debugger">User Guide</a>
        <p>File version: ${ele._fileContext?.version}</p>
      </div>
      <multi-frame-controls-sk></multi-frame-controls-sk>
      <div class="horizontal-flex">
        <commands-sk></commands-sk>
        <div id=center>
          <debug-view-sk></debug-view-sk>
        </div>
        <div id=right>
          ${DebuggerPageSk.controlsTemplate(ele)}
          <!-- hidable gpu op bounds legend-->
          <histogram-sk></histogram-sk>
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

  private static controlsTemplate = (ele: DebuggerPageSk) =>
    html`
    <div>
      <div class="horizontal-flex">
        <checkbox-sk label="GPU" ?checked=${ele._gpuMode}
                     title="Toggle between Skia making WebGL2 calls vs. using it's CPU backend and\
 copying the buffer into a Canvas2D element."
                     @change=${ele._gpuHandler}></checkbox-sk>
        <checkbox-sk label="Display GPU Op Bounds" ?disabled=${!ele._gpuMode}
                     title="Show a visual representation of the GPU operations recorded in each\
 command's audit trail."
                     @change=${ele._opBoundsHandler}></checkbox-sk>
      </div>
      <div class="horizontal-flex">
        <checkbox-sk label="Light/Dark"
                     title="Show transparency backrounds as light or dark"
                     @change=${ele._lightDarkHandler}></checkbox-sk>
        <checkbox-sk label="Display Overdraw Viz"
                     title="Shades pixels redder in proportion to how many times they were written\
 to in the current frame."
                     @change=${ele._overdrawHandler}></checkbox-sk>
      </div>
      <details ?open=${ele._showOpBounds}>
        <summary><b> GPU Op Bounds Legend</b></summary>
        <p style="width: 200px">GPU op bounds are rectangles with a 1 pixel wide stroke. This may\
 mean you can't see them unless you scale the canvas view to its original size.</p>
        <table class=shortcuts>
          <tr><td class=gpuDrawBoundColor>Bounds for the current draw.</td></tr>
          <tr><td class=gpuOpBoundColor>Individual bounds for other draws in the same op.</td></tr>
          <tr><td class=gpuTotalOpColor>Total bounds of the current op.</td></tr>
        </table>
      </details>
      <details open>
        <summary><b>Overlay Options</b></summary>
        <checkbox-sk label="Show Clip"
                     title="Show a semi-transparent teal overlay on the areas within the current\
 clip."
                     id=clip @change=${ele._clipHandler}></checkbox-sk>
        <checkbox-sk label="Show Android Device Clip Restriction"
                     title="Show a semi-transparent peach overlay on the areas within the current\
 andorid device clip restriction.
                     This is set at the beginning of each frame and recorded in the DrawAnnotation\
 Command labeled AndroidDeviceClipRestriction"
                     id=androidclip @change=${ele._androidClipHandler}></checkbox-sk>
        <checkbox-sk label="Show Origin"
                     title="Show the origin of the coordinate space defined by the current matrix."
                     id=origin @change=${ele._originHandler}></checkbox-sk>
        <h3>Clip</h3>
        <table>
          <tr><td>${ ele._info.ClipRect[0] }</td><td>${ ele._info.ClipRect[1] }</td></tr>
          <tr><td>${ ele._info.ClipRect[2] }</td><td>${ ele._info.ClipRect[3] }</td></tr>
        </table>
        <h3>Matrix</h3>
        <table>
          ${ele._matrixTable(ele._info.ViewMatrix)}
        </table>
    </div>`;

  private _skiaVersion: string = 'a url of some kind';
  private _skiaVersionShort: string = '-1';

  // null as long as no file loaded.
  private _fileContext: FileContext | null = null;
  // null until the DebuggerInit promise resolves.
  private _debugger: Debugger | null = null;
  // null until either file loaded or cpu/gpu switch toggled
  private _surface: SkSurface | null = null;

  // submodules are null until first template render
  private _debugViewSk: DebugViewSk | null = null;
  private _commandsSk: CommandsSk | null = null;
  private _playSk: PlaySk | null = null;
  private _zoom: ZoomSk | null = null

  // application state
  private _targetItem: number = 0; // current command playback index in filtered list
  // When turned on, always draw to the end of a frame
  private _drawToEnd: boolean = false;

  // The matrix and clip retrieved from the last draw
  private _info: MatrixClipInfo = {
    ClipRect: [0, 0, 0, 0],
    ViewMatrix: [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
    ],
  };

  // things toggled by the upper right checkboxes.
  private _gpuMode = true; // true means use gpu
  private _showOpBounds = false;
  private _darkBackgrounds = false; // true means dark
  private _showOverdrawViz = false;
  private _showClip = false;
  private _showAndroidClip = false;
  private _showOrigin = false;

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
    this._commandsSk = this.querySelector<CommandsSk>('commands-sk')!;
    this._playSk = this.querySelector<PlaySk>('play-sk')!;
    this._zoom = this.querySelector<ZoomSk>('zoom-sk')!;

    this._zoom.source = this._debugViewSk.canvas;

    this._commandsSk.addEventListener('move-position', (e) => {
      this._targetItem = (e as CustomEvent<CommandsSkMovePositionEventDetail>)
        .detail.position;
      this._updateDebuggerView();
    });

    document.addEventListener('keydown', this._keyDownHandler.bind(this),
      true /* useCapture */);
  }

  // Template helper rendering a number[][] in a table
  private _matrixTable(m: Matrix3x3 | Matrix4x4) {
    return (m as number[][]).map((row: number[]) => {
      return html`<tr>${ row.map((i: number) => html`<td>${i}</td>`) }</tr>`;
    });
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
    const p = playerResult.player;
    this._fileContext = {
      player: p,
      version: p.fileVersion(),
    };
    this._replaceSurface();
    if (!this._surface) {
      errorMessage("Could not create SkSurface, try GPU/CPU toggle.");
      return;
    }
    p.setGpuOpBounds(this._showOpBounds);
    p.setOverdrawVis(this._showOverdrawViz);
    p.setAndroidClipViz(this._showAndroidClip);
    p.setOriginVisible(this._showOrigin);
    this._showClip = false;
    p.setClipVizColor(0);

    this._setCommands();
    // TODO(nifong): Draw the rest of the owl

    this._render();
  }

  // Create a new drawing surface. this should be called when
  // * GPU/CPU mode changes
  // * Bounds of the skp change (skp loaded)
  // * (not yet supported) Color mode changes
  private _replaceSurface() {
    if (!this._debugger) { return; }

    let width = 400;
    let height = 400;
    if (this._fileContext) {
      // From the loaded SKP, player knows how large its picture is. Resize our canvas
      // to match.
      let bounds = this._fileContext.player.getBounds();
      width = bounds.fRight - bounds.fLeft;
      height = bounds.fBottom - bounds.fTop;
      console.log(`SKP width ${width} height ${height}`);
      // Still ok to proceed if no skp, the toggle still should work before a file
      // is picked.
    }
    const canvas = this._debugViewSk!.resize(width, height);
    // free the wasm memory of the previous surface
    if (this._surface) { this._surface.dispose(); }
    if (this._gpuMode) {
      this._surface = this._debugger.MakeWebGLCanvasSurface(canvas);
    } else {
      this._surface = this._debugger.MakeSWCanvasSurface(canvas);
    }
    this._zoom!.source = canvas;
  }

  // Fetch the list of commands for the frame or layer the debugger is currently showing
  // from wasm.
  private _setCommands() {
    // Cache only holds the regular frame's commands, not layers.
    // const json = (self.inspectedLayer === -1 ? this._memoizedJsonCommandList()
    //               : this._player.jsonCommandList(this._surface));
    const json = this._fileContext!.player.jsonCommandList(this._surface!);
    const parsed = JSON.parse(json) as SkpJsonCommandList;
    this._commandsSk!.processCommands(parsed);
  }

  // Asks the wasm module to draw to the provided surface.
  // Up to the command index indidated by this._targetItem
  _updateDebuggerView() {
    if (!this._fileContext) {
      return; // Return early if no file. commands-sk tests load data to that
      // modules but not a whole file.
    }
    if (this._drawToEnd) {
      this._fileContext!.player!.draw(this._surface!);
    } else {
      this._fileContext!.player!.drawTo(this._surface!, this._targetItem);
    }
    if (!this._gpuMode) {
      this._surface!.flush();
    }
    // update zoom
    this._zoom!.update();

    const clipmatjson = this._fileContext.player.lastCommandInfo();
    this._info = JSON.parse(clipmatjson) as MatrixClipInfo;
    this._render();
  }

  // controls change handlers

  private _gpuHandler(e: Event) {
    this._gpuMode = (e.target as CheckOrRadio).checked;
    this._replaceSurface();
    this._setCommands();
  }

  private _lightDarkHandler(e: Event) {
    this._darkBackgrounds = (e.target as CheckOrRadio).checked;
    // should be received by anything in the application that shows a checkerboard
    // background for transparency
    this.dispatchEvent(
      new CustomEvent<DebuggerPageSkLightDarkEventDetail>(
        'light-dark', {
          detail: {mode: this._darkBackgrounds? 'dark-checkerboard' : 'light-checkerboard'},
          bubbles: true,
        }));
  }

  private _opBoundsHandler(e: Event) {
    this._showOpBounds = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setGpuOpBounds(this._showOpBounds);
    this._updateDebuggerView();
  }

  private _overdrawHandler(e: Event) {
    this._showOverdrawViz = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setOverdrawVis(this._showOverdrawViz);
    this._updateDebuggerView();
  }

  private _clipHandler(e: Event) {
    this._showClip = (e.target as CheckOrRadio).checked;
    if(this._showClip) { // ON: 30% transparent dark teal
      this._fileContext!.player.setClipVizColor(parseInt('500e978d',16));
    } else { // OFF: transparent black
      this._fileContext!.player.setClipVizColor(0);
    }
    this._updateDebuggerView();
  }

  private _androidClipHandler(e: Event) {
    this._showAndroidClip = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setAndroidClipViz(this._showAndroidClip);
    this._updateDebuggerView();
  }

  private _originHandler(e: Event) {
    this._showOrigin = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setOriginVisible(this._showOrigin);
    this._updateDebuggerView();
  }

  // Emit both the event that updates zoom, and the one that updates crosshair
  // they're not the same event because the two modules use them to communicate
  // a change in opposite directions and I don't want either of them triggering
  // themselves.
  private _updateBothCursors(x: number, y: number) {
    const p = [x, y] as Point;
    this.dispatchEvent(
      new CustomEvent<ZoomSkPointEventDetail>(
        'zoom-point', {
          detail: {position: p},
          bubbles: true,
        }));
    this.dispatchEvent(
      new CustomEvent<ZoomSkPointEventDetail>(
        'move-zoom-cursor', {
          detail: {position: p},
          bubbles: true,
        }));
  }

  private _keyDownHandler(e: KeyboardEvent) {
    if(this.querySelector<HTMLInputElement>('#text-filter') === document.activeElement) {
      return; // don't interfere with the filter textbox.
    }
    let flen = this._commandsSk!.countFiltered;
    const x = this._zoom!.x;
    const y = this._zoom!.y;
    // If adding a case here, document it in the user-visible keyboard shortcuts area.
    switch (e.keyCode) {
      case 74: // J
        this._updateBothCursors(x, y+1);
        break;
      case 75: // K
        this._updateBothCursors(x, y-1);
        break;
      case 72: // H
        this._updateBothCursors(x-1, y);
        break;
      case 76: // L
        this._updateBothCursors(x+1, y);
        break;
      case 190: // Period, step command forward
        this._commandsSk!.keyMove(1);
        break;
      case 188: // Comma, step command back
        this._commandsSk!.keyMove(-1);
        break;
      // case 87: // w
      //   if (this.$.frames_slider.disabled) { return; }
      //   this._moveFrameTo(this.frameIndex-1);
      //   break;
      // case 83: // s
      //   if (this.$.frames_slider.disabled) { return; }
      //   this._moveFrameTo(this.frameIndex+1);
      //   break;
      // case 80: // p
      //   this._toggleFramePlay();
      //   break;
      default:
        return;
    }
    e.stopPropagation();
  }
};

define('debugger-page-sk', DebuggerPageSk);
