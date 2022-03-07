/**
 * @module modules/debugger-page-sk
 * @description The main module of the wasm-based SKP and MSKP debugger.
 *  Holds the loaded wasm module, pointer to the Surface in wasm, and SKP file state.
 *  Handles all the interaction and control of the application that does not cleanly fit
 *  within a submodule.
 *
 * @evt render-cursor: Emitted when the cursor changes position, or the data under it changes.
 *      There are three modules which can both change, and render representatoins of the cursor.
 *      - debugger-page-sk: Can change the cursor with keypresses, can change the data under
 *                          the cursor, uses the cursor to provide a jump-to-command feature.
 *      - debug-view-sk: Can change the cursor by clicking on the canvas, or mousing over the
 *                       canvas. Shows a visual crosshair to represent the cursor.
 *      - zoom-sk: Can change the cursor by clicking pixels on the zoom canvas. Shows data that
 *                 depends on the cursor location and data under the cursor.
 *      To solve this coordination problem, everything is sent through debugger-page-sk
 *      When the zoom or debug-view modules move the cursor, they emit move-cursor but
 *      don't render. debugger-page-sk receives this, renders itself, and re-emits render-cursor,
 *      which zoom, and debug-view consume. If the change originates from debugger-page-sk,
 *      it only emits render-cursor.
 *
 *       [               ] --- move-cursor --> [           ] <-- move-cursor --- [         ]
 *       [ debug-view-sk ]                     [ debugger- ]                     [ zoom-sk ]
 *       [               ] <- render-cursor -- [ page-sk   ] -- render-cursor -> [         ]
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';
import { TabsSk } from 'elements-sk/tabs-sk/tabs-sk';
import 'elements-sk/checkbox-sk';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { ElementDocSk } from '../element-doc-sk/element-doc-sk';
import { DebugViewSk } from '../debug-view-sk/debug-view-sk';
import { CommandsSk } from '../commands-sk/commands-sk';
import { TimelineSk } from '../timeline-sk/timeline-sk';
import { PlaySk } from '../play-sk/play-sk';
import { ZoomSk } from '../zoom-sk/zoom-sk';
import 'elements-sk/error-toast-sk';
import { AndroidLayersSk } from '../android-layers-sk/android-layers-sk';
import { ResourcesSk } from '../resources-sk/resources-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

// Types for the wasm bindings
import type {
  Canvas,
  CanvasKit,
  CanvasKitInitOptions,
  Paint,
  Surface,
} from '../../wasm_libs/types/canvaskit';
import {
  Debugger,
  Matrix3x3,
  Matrix4x4,
  MatrixClipInfo,
  SkIRect,
  SkpDebugPlayer,
  SkpJsonCommandList,
} from '../debugger';

// other modules from this application
import '../android-layers-sk';
import '../commands-sk';
import '../debug-view-sk';
import '../histogram-sk';
import '../resources-sk';
import '../timeline-sk';
import '../zoom-sk';
import {
  InspectLayerEventDetail,
  CursorEventDetail,
  ToggleBackgroundEventDetail,
  InspectLayerEvent,
  JumpCommandEvent,
  JumpCommandEventDetail,
  ModeChangedManuallyEvent,
  MoveCommandPositionEvent,
  MoveCommandPositionEventDetail,
  MoveCursorEvent,
  MoveFrameEvent,
  Point,
  RenderCursorEvent,
  SelectImageEvent,
  SelectImageEventDetail,
  ToggleBackgroundEvent, ModeChangedManuallyEventDetail, MoveFrameEventDetail,
} from '../events';

// Declarations for variables defined in JS files included by main.html;
// It is assumed that canvaskit.js has been loaded and this symbol is available globally.
declare function CanvasKitInit(opts: CanvasKitInitOptions): Promise<Debugger>;
declare const SKIA_VERSION: string;

interface FileContext {
  player: SkpDebugPlayer;
  version: number;
  frameCount: number;
}

export class DebuggerPageSk extends ElementDocSk {
  private static template = (ele: DebuggerPageSk) => html`
  <app-sk>
    <header>
      <h2>Skia WASM Debugger</h2>
      <span>
        <a class="version-link"
          href="https://skia.googlesource.com/skia/+show/${ele._skiaVersion}"
          title="The skia commit at which the debugger WASM module was built">
          ${ele._skiaVersionShort}
        </a>
        &nbsp;
        <a href="/versions">Older Debugger Versions</a>
        <theme-chooser-sk></theme-chooser-sk>
      </span>
    </header>
    <main id=content>
      <div class="horizontal-flex">
        <label>SKP to open:</label>
        <input type="file" @change=${ele._fileInputChanged}
         ?disabled=${ele._debugger === null} />
        <a href="https://skia.org/dev/tools/debugger">User Guide</a>
        <p class="file-version">File version: ${ele._fileContext?.version}</p>
        <p class="file-version">Minimum version this build can open: ${ele._debugger?.MinVersion()}</p>
      </div>
      <timeline-sk></timeline-sk>
      <div class="horizontal-flex-start">
        <commands-sk></commands-sk>
        <div id=center>
          <tabs-sk id='center-tabs'>
            <button class=selected>SKP</button>
            <button>Image Resources</button>
          </tabs-sk>
          <tabs-panel-sk>
            <div>
              <debug-view-sk></debug-view-sk>
            </div>
            <div>
              <resources-sk></resources-sk>
            </div>
          </tabs-panel-sk>
        </div>
        <div id=right>
          ${DebuggerPageSk.controlsTemplate(ele)}
          <histogram-sk></histogram-sk>
          <div>Command which shaded the<br>selected pixel: ${ele._pointCommandIndex}
            <button @click=${() => {
    ele._jumpToCommand(ele._pointCommandIndex);
  }}>Jump</button>
          </div>
          <zoom-sk></zoom-sk>
          <android-layers-sk></android-layers-sk>
        </div>
      </div>
    </main>
    <footer>
      <error-toast-sk></error-toast-sk>
    </footer>
  </app-sk>
    `;

  private static controlsTemplate = (ele: DebuggerPageSk) => html`
    <div>
      <table>
        <tr>
          <td><checkbox-sk label="GPU" ?checked=${ele._gpuMode}
               title="Toggle between Skia making WebGL2 calls vs. using it's CPU backend and\
 copying the buffer into a Canvas2D element."
                       @change=${ele._gpuHandler}></checkbox-sk></td>
          <td><checkbox-sk label="Display GPU Op Bounds" ?disabled=${!ele._gpuMode}
               title="Show a visual representation of the GPU operations recorded in each\
 command's audit trail."
                       @change=${ele._opBoundsHandler}></checkbox-sk></td>
        </tr>
        <tr>
          <td><checkbox-sk label="Light/Dark"
               title="Show transparency backrounds as light or dark"
                       @change=${ele._lightDarkHandler}></checkbox-sk></td>
          <td><checkbox-sk label="Display Overdraw Viz"
               title="Shades pixels redder in proportion to how many times they were written\
 to in the current frame."
                       @change=${ele._overdrawHandler}></checkbox-sk></td>
        </tr>
      </table>
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
        <div class="horizontal-flex">
          <div class="matrixClipBox">
            <h3 class="compact">Clip</h3>
            <table>
              <tr><td>${ele._info.ClipRect[0]}</td><td>${ele._info.ClipRect[1]}</td></tr>
              <tr><td>${ele._info.ClipRect[2]}</td><td>${ele._info.ClipRect[3]}</td></tr>
            </table>
          </div>
          <div class="matrixClipBox">
            <h3 class="compact">Matrix</h3>
            <table>
              ${ele._matrixTable(ele._info.ViewMatrix)}
            </table>
          </div>
        </div>
      </details>
    </div>`;

  // defined by version.js which is included by main.html and generated in Makefile.
  private _skiaVersion: string = SKIA_VERSION;

  private _skiaVersionShort: string = SKIA_VERSION.substring(0, 7);

  // null as long as no file loaded.
  private _fileContext: FileContext | null = null;

  // null until the DebuggerInit promise resolves.
  private _debugger: Debugger | null = null;

  // null until either file loaded or cpu/gpu switch toggled
  private _surface: Surface | null = null;

  // submodules are null until first template render
  private _androidLayersSk: AndroidLayersSk | null = null;

  private _debugViewSk: DebugViewSk | null = null;

  private _commandsSk: CommandsSk | null = null;

  private _resourcesSk: ResourcesSk | null = null;

  private _timelineSk: TimelineSk | null = null;

  private _zoom: ZoomSk | null = null

  // application state
  private _targetItem: number = 0; // current command playback index in filtered list

  // When turned on, always draw to the end of a frame
  private _drawToEnd: boolean = false;

  // the index of the last command to alter the pixel under the crosshair
  private _pointCommandIndex = 0;

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

    CanvasKitInit({
      locateFile: (file: string) => `/dist/${file}`,
    }).then((loadedWasmModule) => {
      // Save a reference to the module somewhere we can use it later.
      this._debugger = loadedWasmModule;
      // File input element should now be enabled, so we need to render.
      this._render();
      // It is now possible to load SKP files.
      this._checkUrlParams();
    });
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._androidLayersSk = this.querySelector<AndroidLayersSk>('android-layers-sk')!;
    this._debugViewSk = this.querySelector<DebugViewSk>('debug-view-sk')!;
    this._commandsSk = this.querySelector<CommandsSk>('commands-sk')!;
    this._resourcesSk = this.querySelector<ResourcesSk>('resources-sk')!;
    this._timelineSk = this.querySelector<TimelineSk>('timeline-sk')!;
    this._zoom = this.querySelector<ZoomSk>('zoom-sk')!;

    this._zoom.source = this._debugViewSk.canvas;

    this._commandsSk.addEventListener(MoveCommandPositionEvent, (e) => {
      const detail = (e as CustomEvent<MoveCommandPositionEventDetail>).detail;
      this._targetItem = detail.position;
      this._updateDebuggerView();
      if (detail.paused) {
        this._updateJumpButton(this._zoom!.point);
      }
    });

    this._timelineSk.playsk.addEventListener(
      ModeChangedManuallyEvent, (e) => {
        const mode = (e as CustomEvent<ModeChangedManuallyEventDetail>).detail.mode;
        if (mode === 'pause') {
          this._setCommands();
        }
      },
    );

    // this is the timeline, which owns the frame position, telling this element to update
    this._timelineSk.addEventListener(MoveFrameEvent, (e) => {
      const frame = (e as CustomEvent<MoveFrameEventDetail>).detail.frame;
      this._moveFrameTo(frame);
    });

    // this is the command list telling us to show the resource viewer and select an image
    this._commandsSk.addEventListener(SelectImageEvent, (e) => {
      this.querySelector<TabsSk>('tabs-sk')!.select(1);
      const id = (e as CustomEvent<SelectImageEventDetail>).detail.id;
      this._resourcesSk!.selectItem(id, true);
    });

    this._androidLayersSk.addEventListener(InspectLayerEvent, (e) => {
      const detail = (e as CustomEvent<InspectLayerEventDetail>).detail;
      this._inspectLayer(detail.id, detail.frame);
      this.querySelector<TabsSk>('tabs-sk')!.select(0);
    });

    this.addDocumentEventListener('keydown', this._keyDownHandler.bind(this),
      true /* useCapture */);

    this.addDocumentEventListener(MoveCursorEvent, (e) => {
      const detail = (e as CustomEvent<CursorEventDetail>).detail;
      // Update this module's cursor-dependent element(s)
      this._updateJumpButton(detail.position);
      // re-emit event as render-cursor
      this.dispatchEvent(
        new CustomEvent<CursorEventDetail>(
          RenderCursorEvent, {
            detail: detail,
            bubbles: true,
          },
        ),
      );
    });
  }

  private _checkUrlParams(): void {
    const params = new URLSearchParams(window.location.search);
    if (params.has('url')) {
      const skpurl = params.get('url')!;
      fetch(skpurl).then((response) => response.arrayBuffer()).then((ab) => {
        if (ab) {
          this._openSkpFile(ab);
        } else {
          errorMessage(`No data received from ${skpurl}`);
        }
      });
    }
  }

  // Searches for the command which left the given pixel in it's current color,
  // Updates the Jump button with the result.
  // Consider disabling this feature alltogether for CPU backed debugging, too slow.
  private _updateJumpButton(p: Point): void {
    if (!this._debugViewSk!.crosshairActive) {
      return; // Too slow to do this on every mouse move.
    }
    this._pointCommandIndex = this._fileContext!.player.findCommandByPixel(
      this._surface!, p[0], p[1], this._targetItem,
    );
    this._render();
  }

  // Template helper rendering a number[][] in a table
  private _matrixTable(m: Matrix3x3 | Matrix4x4): TemplateResult[] {
    return (m as number[][]).map((row: number[]) => html`<tr>${row.map((i: number) => html`<td>${i}</td>`)}</tr>`);
  }

  // Called when the filename in the file input element changs
  private _fileInputChanged(e: Event): void {
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

  // Finds the version number from the binary SKP or MSKP file format
  // This is done in JS because to avoid any chance of SkpFilePlayer crashing on old
  // versions and not reporting back with a version number.
  // Overall it was just a lot simpler to do it here.
  private _skpFileVersion(fileContents: ArrayBuffer): number {
    const utf8decoder = new TextDecoder();
    // only check up to 2000 bytes
    // MSKP files have a variable sized header before the internal SKP with the version
    // number we're looking for.
    const head = new Uint8Array(fileContents).subarray(0, 2000);
    function isMagicWord(element: number, index: number, array: Uint8Array) {
      return utf8decoder.decode(array.subarray(index, index + 8)) === 'skiapict';
    }
    // Note that we want to locate the offset in a Uint8Array, not in a string that
    // might interpret binary stuff before "skiapict" as multi-byte code points.
    const magicOffset = head.findIndex(isMagicWord);
    // The unint32 after the first occurance of "skiapict" is the SKP version
    return new Uint32Array(head.subarray(magicOffset + 8, magicOffset + 12))[0];
  }

  // Open an SKP or MSKP file. fileContents is expected to be an arraybuffer
  // with the file's contents
  private _openSkpFile(fileContents: ArrayBuffer): void {
    if (!this._debugger) { return; }
    const version = this._skpFileVersion(fileContents);
    const minVersion = this._debugger.MinVersion();
    if (version < minVersion) {
      errorMessage(`File version (${version}) is older than this skia build's minimum\
 supported version (${minVersion}). Debugger may crash if the file contains unreadable sections.`);
    }
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
      version: version,
      frameCount: p.getFrameCount(),
    };
    this._replaceSurface();
    if (!this._surface) {
      errorMessage('Could not create Surface, try GPU/CPU toggle.');
      return;
    }
    p.setGpuOpBounds(this._showOpBounds);
    p.setOverdrawVis(this._showOverdrawViz);
    p.setAndroidClipViz(this._showAndroidClip);
    p.setOriginVisible(this._showOrigin);
    this._showClip = false;
    p.setClipVizColor(0);

    console.log(`Loaded SKP file with ${this._fileContext.frameCount} frames`);
    this._resourcesSk!.reset();
    // Determine if we loaded a single-frame or multi-frame SKP.
    if (this._fileContext.frameCount > 1) {
      this._timelineSk!.count = this._fileContext.frameCount;
      this._timelineSk!.hidden = false;
      // shared images deserialproc only used with mskps
      this._resourcesSk!.update(p);
    } else {
      this._timelineSk!.hidden = true;
    }

    // Pull the command list for the first frame.
    // triggers render
    this._setCommands();
    this._androidLayersSk!.update(this._commandsSk!.layerInfo,
      this._fileContext!.player.getLayerSummariesJs(), 0);
  }

  // Create a new drawing surface. this should be called when
  // * GPU/CPU mode changes
  // * Bounds of the skp change (skp loaded)
  // * (not yet supported) Color mode changes
  private _replaceSurface(): void {
    if (!this._debugger) { return; }

    let width = 400;
    let height = 400;
    if (this._fileContext) {
      // From the loaded SKP, player knows how large its picture is. Resize our canvas
      // to match.
      const bounds = this._fileContext.player.getBounds();
      width = bounds.fRight - bounds.fLeft;
      height = bounds.fBottom - bounds.fTop;
      // Still ok to proceed if no skp, the toggle still should work before a file
      // is picked.
    }
    this._replaceSurfaceKnownBounds(width, height);
  }

  // replace surface, using the given size
  private _replaceSurfaceKnownBounds(width: number, height: number): void {
    if (!this._debugger) { return; }
    const canvas = this._debugViewSk!.resize(width, height);
    // free the wasm memory of the previous surface
    if (this._surface) { this._surface.dispose(); }
    if (this._gpuMode) {
      let glAttrs = {
        // for the zoom to be able to access the pixels. Incurs performance penalty
        'preserveDrawingBuffer': 1,
      };
      this._surface = this._debugger.MakeWebGLCanvasSurface(canvas,
        this._debugger.ColorSpace.SRGB, glAttrs);
    } else {
      this._surface = this._debugger.MakeSWCanvasSurface(canvas);
    }
    this._zoom!.source = canvas;
  }

  // Moves the player to a frame and updates dependent elements
  // Note that if you want to move the frame for the whole app, just as if a user did it,
  // this is not the function you're looking for, instead set this._timelineSk.item
  private _moveFrameTo(n: number): void {
    // bounds may change too, requiring a new surface and gl context, but this is costly and
    // only rarely necessary
    const oldBounds = this._fileContext!.player.getBounds();
    const newBounds = this._fileContext!.player.getBoundsForFrame(n);
    if (!this._boundsEqual(oldBounds, newBounds)) {
      const width = newBounds.fRight - newBounds.fLeft;
      const height = newBounds.fBottom - newBounds.fTop;
      this._replaceSurfaceKnownBounds(width, height);
    } else if (this._surface) {
      // When not changing size, it only needs to be cleared.
      this._surface.getCanvas().clear(this._debugger!.TRANSPARENT);
    }

    this._fileContext!.player.changeFrame(n);
    // If the frame moved and the state is paused, also update the command list
    const mode = this._timelineSk!.querySelector<PlaySk>('play-sk')!.mode;
    if (mode === 'pause') {
      this._setCommands();
      this._androidLayersSk!.update(this._commandsSk!.layerInfo,
        this._fileContext!.player.getLayerSummariesJs(), n);
    } else {
      this._updateDebuggerView();
    }
    this._timelineSk!.playsk.movedTo(n);
  }

  private _boundsEqual(a: SkIRect, b: SkIRect): boolean {
    return (a.fLeft === b.fLeft
         && a.fTop === b.fTop
         && a.fRight === b.fRight
         && a.fBottom === b.fBottom);
  }

  // Fetch the list of commands for the frame or layer the debugger is currently showing
  // from wasm.
  private _setCommands(): void {
    // Cache only holds the regular frame's commands, not layers.
    // const json = (self.inspectedLayer === -1 ? this._memoizedJsonCommandList()
    //               : this._player.jsonCommandList(this._surface));
    const json = this._fileContext!.player.jsonCommandList(this._surface!);
    const parsed = JSON.parse(json) as SkpJsonCommandList;
    // this will eventually cause a move-command-position event
    this._commandsSk!.processCommands(parsed);
  }

  private _jumpToCommand(i: number): void {
    // listened to by commands-sk
    this.dispatchEvent(
      new CustomEvent<JumpCommandEventDetail>(
        JumpCommandEvent, {
          detail: { unfilteredIndex: i },
          bubbles: true,
        },
      ),
    );
  }

  // Asks the wasm module to draw to the provided surface.
  // Up to the command index indidated by this._targetItem
  _updateDebuggerView(): void {
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
    this.dispatchEvent(
      new CustomEvent<CursorEventDetail>(
        RenderCursorEvent, {
          detail: { position: [0, 0], onlyData: true },
          bubbles: true,
        },
      ),
    );

    const clipmatjson = this._fileContext.player.lastCommandInfo();
    this._info = JSON.parse(clipmatjson) as MatrixClipInfo;
    this._render();
  }

  // controls change handlers

  private _gpuHandler(e: Event): void {
    this._gpuMode = (e.target as CheckOrRadio).checked;
    this._replaceSurface();
    if (!this._surface) {
      errorMessage('Could not create Surface.');
      return;
    }
    this._setCommands();
  }

  private _lightDarkHandler(e: Event): void {
    this._darkBackgrounds = (e.target as CheckOrRadio).checked;
    // should be received by anything in the application that shows a checkerboard
    // background for transparency
    this.dispatchEvent(
      new CustomEvent<ToggleBackgroundEventDetail>(
        ToggleBackgroundEvent, {
          detail: { mode: this._darkBackgrounds ? 'dark-checkerboard' : 'light-checkerboard' },
          bubbles: true,
        },
      ),
    );
  }

  private _opBoundsHandler(e: Event): void {
    this._showOpBounds = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setGpuOpBounds(this._showOpBounds);
    this._updateDebuggerView();
  }

  private _overdrawHandler(e: Event): void {
    this._showOverdrawViz = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setOverdrawVis(this._showOverdrawViz);
    this._updateDebuggerView();
  }

  private _clipHandler(e: Event): void {
    this._showClip = (e.target as CheckOrRadio).checked;
    if (this._showClip) { // ON: 30% transparent dark teal
      this._fileContext!.player.setClipVizColor(0x500e978d);
    } else { // OFF: transparent black
      this._fileContext!.player.setClipVizColor(0);
    }
    this._updateDebuggerView();
  }

  private _androidClipHandler(e: Event): void {
    this._showAndroidClip = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setAndroidClipViz(this._showAndroidClip);
    this._updateDebuggerView();
  }

  private _originHandler(e: Event): void {
    this._showOrigin = (e.target as CheckOrRadio).checked;
    this._fileContext!.player.setOriginVisible(this._showOrigin);
    this._updateDebuggerView();
  }

  private _updateCursor(x: number, y: number): void {
    this._updateJumpButton([x, y]);
    this.dispatchEvent(
      new CustomEvent<CursorEventDetail>(
        RenderCursorEvent, {
          detail: { position: [x, y], onlyData: false },
          bubbles: true,
        },
      ),
    );
  }

  private _keyDownHandler(e: KeyboardEvent): void {
    if (this.querySelector<HTMLInputElement>('#text-filter') === document.activeElement) {
      return; // don't interfere with the filter textbox.
    }
    const [x, y] = this._zoom!.point;
    // If adding a case here, document it in the user-visible keyboard shortcuts area.
    switch (e.keyCode) {
      case 74: // J
        this._updateCursor(x, y + 1);
        break;
      case 75: // K
        this._updateCursor(x, y - 1);
        break;
      case 72: // H
        this._updateCursor(x - 1, y);
        break;
      case 76: // L
        this._updateCursor(x + 1, y);
        break;
      case 190: // Period, step command forward
        this._commandsSk!.keyMove(1);
        break;
      case 188: // Comma, step command back
        this._commandsSk!.keyMove(-1);
        break;
      case 87: // w
        this._timelineSk!.playsk.prev();
        break;
      case 83: // s
        this._timelineSk!.playsk.next();
        break;
      case 80: // p
        this._timelineSk!.playsk.togglePlay();
        break;
      default:
        return;
    }
    e.stopPropagation();
  }

  private _inspectLayer(layerId: number, frame: number): void {
    // This method is called any time one of the Inspector/Exit buttons is pressed.
    // if the the button was on the layer already being inspected, it says "exit"
    // and -1 is passed to layerId.
    // It is also called from the jump action in the image resorce viewer.
    // TODO(nifong): Either disable the timeline or make it have some kind of layer-aware
    // mode that would jump between updates. At the moment if you move the frame while viewing
    // a layer, you'll bork the app.
    this._timelineSk!.item = frame;
    this._fileContext!.player.setInspectedLayer(layerId);
    this._replaceSurface();
    this._setCommands();
    this._commandsSk!.end();
  }
}

define('debugger-page-sk', DebuggerPageSk);
