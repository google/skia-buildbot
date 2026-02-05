/**
 * @module skottie-sk
 * @description <h2><code>skottie-sk</code></h2>
 *
 * <p>
 *   The main application element for skottie.
 * </p>
 *
 */
import '../skottie-config-sk';
import '../skottie-player-sk';
import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/collapse-sk';
import '../../../elements-sk/modules/error-toast-sk';
import Ajv from 'ajv/dist/2020';
import { html, TemplateResult } from 'lit/html.js';
import {
  JSONEditor,
  toJSONContent,
  createAjvValidator,
  JSONEditorPropsOptional,
} from 'vanilla-jsoneditor';
import LottiePlayer from 'lottie-web';
import { RendererType } from 'lottie-web';
import { $$ } from '../../../infra-sk/modules/dom';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { CollapseSk } from '../../../elements-sk/modules/collapse-sk/collapse-sk';
import { SkottieGifExporterSk } from '../skottie-gif-exporter-sk/skottie-gif-exporter-sk';
import '../skottie-gif-exporter-sk';
import '../skottie-text-editor-sk';
import '../skottie-library-sk';
import { SoundMap, AudioPlayer } from '../audio';
import '../skottie-performance-sk';
import '../skottie-compatibility-sk';
import { renderByDomain } from '../helpers/templates';
import { isDomain } from '../helpers/domains';
import '../skottie-audio-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  SkottieConfigEventDetail,
  SkottieConfigState,
} from '../skottie-config-sk/skottie-config-sk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';
import { SkottiePerformanceSk } from '../skottie-performance-sk/skottie-performance-sk';
import { FontAsset, LottieAnimation, LottieAsset, ViewMode } from '../types';
import { SkottieLibrarySk } from '../skottie-library-sk/skottie-library-sk';
import { AudioStartEventDetail, SkottieAudioSk } from '../skottie-audio-sk/skottie-audio-sk';
import {
  SkottieTextEditorSk,
  TextEditEventDetail,
} from '../skottie-text-editor-sk/skottie-text-editor-sk';
import '../skottie-shader-editor-sk';
import {
  ShaderEditApplyEventDetail,
  ShaderEditorSk,
} from '../skottie-shader-editor-sk/skottie-shader-editor-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import { replaceShaders } from '../skottie-shader-editor-sk/shader-replace';
import '../../../elements-sk/modules/icons/expand-less-icon-sk';
import '../../../elements-sk/modules/icons/expand-more-icon-sk';
import '../../../elements-sk/modules/icons/play-arrow-icon-sk';
import '../../../elements-sk/modules/icons/pause-icon-sk';
import '../../../elements-sk/modules/icons/replay-icon-sk';
import '../../../elements-sk/modules/icons/file-download-icon-sk';
import '../skottie-button-sk';
import '../skottie-dropdown-sk';
import { DropdownSelectEvent } from '../skottie-dropdown-sk/skottie-dropdown-sk';
import '../skottie-exporter-sk';
import { ExportType, SkottieExporterSk } from '../skottie-exporter-sk/skottie-exporter-sk';
import '../skottie-file-settings-sk';
import {
  SkottieFileSettingsSk,
  SkottieFileSettingsEventDetail,
} from '../skottie-file-settings-sk/skottie-file-settings-sk';
import '../skottie-file-form-sk';
import { SkottieFilesEventDetail } from '../skottie-file-form-sk/skottie-file-form-sk';
import '../skottie-background-settings-sk';
import { SkottieBackgroundSettingsEventDetail } from '../skottie-background-settings-sk/skottie-background-settings-sk';
import '../skottie-color-manager-sk';
import '../skottie-slot-manager-sk';
import { SkottieTemplateEventDetail } from '../skottie-color-manager-sk/skottie-color-manager-sk';
import { isBinaryAsset } from '../helpers/animation';
import '../window/window';
import { SkottieSlotManagerSk } from '../skottie-slot-manager-sk/skottie-slot-manager-sk';
import { lottieSchema } from '../skottie-compatibility-sk/schemas/lottie.schema';

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

interface BodymovinPlayer {
  goToAndStop(t: number): void;
  goToAndPlay(t: number): void;
  pause(): void;
  play(): void;
  destroy(): void;
}

interface LoadedAsset {
  name: string;
  bytes?: ArrayBuffer;
  player?: AudioPlayer;
}

const GOOGLE_WEB_FONTS_HOST = 'https://cdn.skia.org/google-web-fonts';

const PRODUCTION_ASSETS_PATH = '/_/a';

// Make this the hash of the lottie file you want to play on startup.
const DEFAULT_LOTTIE_FILE = '5c1c5cc9aa4aabe4acc1f12a7bac60fb'; // gear.json

// SCRUBBER_RANGE is the input range for the scrubbing control.
// This is an arbitrary value, and is treated as a re-scaled duration.
const SCRUBBER_RANGE = 1000;

// window.skottie is null in tests.
const SUPPORTED_DOMAINS = {
  SKOTTIE_INTERNAL: window.skottie
    ? window.skottie.internal_site_domain
    : 'skottie-internal.corp.goog',
  SKOTTIE_TENOR: window.skottie ? window.skottie.tenor_site_domain : 'skottie-tenor.corp.goog',
  SKOTTIE: window.skottie ? window.skottie.public_site_domain : 'skottie.skia.org',
  LOCALHOST: 'localhost',
};

const AUDIO_SUPPORTED_DOMAINS = [
  SUPPORTED_DOMAINS.SKOTTIE_INTERNAL,
  SUPPORTED_DOMAINS.SKOTTIE_TENOR,
  SUPPORTED_DOMAINS.LOCALHOST,
];

type UIMode = 'loading' | 'loaded' | 'idle' | 'draft' | 'unsynced' | 'synced';

type ToolType =
  | 'none'
  | 'skottie-library'
  | 'text-edits'
  | 'shader-edits'
  | 'background-color'
  | 'json-editor'
  | 'color-manager'
  | 'skottie-font'
  | 'skottie-player'
  | 'lottie-player'
  | 'slot-manager';

const caption = (text: string, mode: ViewMode) => {
  if (mode === 'presentation') {
    return null;
  }
  return html` <figcaption>${text}</figcaption> `;
};

const redir = () =>
  renderByDomain(
    html` <div>
      Googlers should use
      <a href="https://${SUPPORTED_DOMAINS.SKOTTIE_INTERNAL}"
        >${SUPPORTED_DOMAINS.SKOTTIE_INTERNAL}</a
      >.
    </div>`,
    Object.values(SUPPORTED_DOMAINS).filter(
      (domain: string) => domain !== SUPPORTED_DOMAINS.SKOTTIE_INTERNAL
    )
  );

const displayLoading = () => html`
  <div class="loading"><spinner-sk active></spinner-sk><span>Loading...</span></div>
`;

export class SkottieSk extends ElementSk {
  private static template = (ele: SkottieSk) => html`
    <app-sk>
      <div class="app-container">
        <header>
          <h2>Skottie Web Player</h2>
          <span>
            <a
              href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
              class="header__skia-version">
              ${SKIA_VERSION.slice(0, 7)}
            </a>

            <skottie-dropdown-sk
              id="view-exporter"
              .name="dropdown-exporter"
              .options=${[
                { id: '', value: 'Export' },
                { id: 'gif', value: 'GIF' },
                { id: 'webM', value: 'WebM' },
                { id: 'png', value: 'PNG sequence' },
              ]}
              reset
              @select=${ele.exportSelectHandler}
              border>
            </skottie-dropdown-sk>
            <skottie-button-sk
              id="view-perf-chart"
              @select=${ele.togglePerformanceChart}
              type="outline"
              .content=${'Performance chart'}
              .classes=${['header__button', ele.showPerformanceChart ? 'active-dialog' : '']}>
            </skottie-button-sk>
            ${ele.compatibilityReportOpen()}
            <skottie-button-sk
              id="view-json-layers"
              @select=${ele.toggleEditor}
              type="outline"
              .content=${'View JSON code'}
              .classes=${['header__button', ele.showJSONEditor ? 'active-dialog' : '']}>
            </skottie-button-sk>
            ${ele.renderApplyChanges()}

            <theme-chooser-sk></theme-chooser-sk>
          </span>
        </header>
        <main>${ele.pick()}</main>
        <footer>
          <error-toast-sk></error-toast-sk>
          ${redir()}
        </footer>
      </div>
      <skottie-exporter-sk
        @start=${ele.onExportStart}
        .downloadFileName=${ele.state.filename || 'Download'}>
      </skottie-exporter-sk>
    </app-sk>
  `;

  // pick the right part of the UI to display based on ele._ui.
  private pick = () => {
    switch (this.ui) {
      default:
      case 'idle':
        return this.displayIdle();
      case 'loading':
        return displayLoading();
      case 'loaded':
      case 'unsynced':
      case 'synced':
      case 'draft':
        return this.displayLoaded();
    }
  };

  private renderApplyChanges = (): TemplateResult | null => {
    if (!this.areChangesUploaded()) {
      return html`<skottie-button-sk
        id="view-json-layers"
        @select=${this.applyEdits}
        type="outline"
        .content=${'Save all changes'}
        .classes=${['header__button']}>
      </skottie-button-sk>`;
    }
    return null;
  };

  private displayDialog = () => html`
    <skottie-config-sk
      .state=${this.state}
      .width=${this.width}
      .height=${this.height}
      .fps=${this.fps}
      .backgroundColor=${this.backgroundColor}
      @skottie-selected=${this.skottieFileSelected}
      @cancelled=${this.selectionCancelled}></skottie-config-sk>
  `;

  private displayIdle = () => html`
    <div class="threecol">
      <div class="left-panel">${this.leftControls()}</div>
      <div class="main-panel"></div>
      <div class="right-panel">${this.rightControls()}</div>
    </div>
  `;

  private displayLoaded = () => html`
    <div class="threecol">
      <div class="left-panel">${this.leftControls()}</div>
      <div class="main-panel">${this.mainContent()}</div>
      <div class="right-panel">${this.rightControls()}</div>
    </div>
  `;

  private mainContent = () => html`
    <div class="players">
      <figure class="players-container">
        ${this.skottiePlayerTemplate()} ${this.lottiePlayerTemplate()}
      </figure>
    </div>
    <div class="playback">
      <div class="playback-content">
        <skottie-button-sk
          id="playpause"
          .content=${html`<play-arrow-icon-sk id="playpause-play"></play-arrow-icon-sk>
            <pause-icon-sk id="playpause-pause"></pause-icon-sk>`}
          .classes=${['playback-content__button']}
          @select=${this.playpause}></skottie-button-sk>
        <div class="scrub">
          <input
            id="scrub"
            type="range"
            min="0"
            max=${SCRUBBER_RANGE}
            step="0.1"
            @input=${this.onScrub}
            @change=${this.onScrubEnd} />
          <label class="number">
            Frame:
            <input
              type="number"
              id="frameInput"
              class="playback-content-frameInput"
              @focus=${this.onFrameFocus}
              @change=${this.onFrameChange} /><!--
            --><span class="playback-content-frameTotal" id="frameTotal">of 0</span>
          </label>
        </div>
        <skottie-button-sk
          id="rewind"
          .content=${html`<replay-icon-sk></replay-icon-sk>`}
          .classes=${['playback-content__button']}
          @select=${this.rewind}></skottie-button-sk>
      </div>
    </div>

    <div @click=${this.onChartClick}>${this.performanceChartTemplate()}</div>
    ${this.jsonEditor()} ${this.gifExporter()} ${this.compatibilityReportTemplate()}

    <collapse-sk id="volume" closed>
      <p>Volume:</p>
      <input
        id="volume-slider"
        type="range"
        min="0"
        max="1"
        step=".05"
        value="1"
        @input=${this.onVolumeChange} />
    </collapse-sk>
  `;

  private embedDialog() {
    return html`
      <details class="embed expando">
        <summary id="embed-open">
          <span>Embed</span><expand-less-icon-sk></expand-less-icon-sk>
          <expand-more-icon-sk></expand-more-icon-sk>
        </summary>
        <label>
          Embed using an iframe
          <input value=${this.iframeDirections()} />
        </label>
        <label>
          Embed on skia.org
          <input value=${this.inlineDirections()} />
        </label>
      </details>
    `;
  }

  private slotManager() {
    return html`
      <details class="expando">
        <summary>
          <span>Slot manager</span>
          <expand-less-icon-sk></expand-less-icon-sk>
          <expand-more-icon-sk></expand-more-icon-sk>
        </summary>
        <skottie-slot-manager-sk
          .player=${this.skottiePlayer}
          .animation=${this.state.lottie}
          @slot-manager-change=${this.onSlotManagerUpdated}>
        </skottie-slot-manager-sk>
      </details>
    `;
  }

  private compatibilityReportOpen() {
    return html` <skottie-button-sk
      id="view-compatibility-report"
      @select=${this.toggleCompatibilityReport}
      type="outline"
      .content=${'Compatibility Report (Beta)'}
      .classes=${['header__button', this.showCompatibilityReport ? 'active-dialog' : '']}>
    </skottie-button-sk>`;
  }

  private compatibilityReportTemplate() {
    return html`
      <dialog class="compatibility-report" ?open=${this.showCompatibilityReport}>
        <div class="top-ribbon">
          <span>Compatibility Report</span>
          <button @click=${this.toggleCompatibilityReport}>Close</button>
        </div>
        <skottie-compatibility-sk
          .animation=${this.state.lottie}
          @updateAnimation=${this.updateAnimation}>
        </skottie-compatibility-sk>
      </dialog>
    `;
  }

  private performanceChartTemplate() {
    return html`
      <dialog class="perf-chart" ?open=${this.showPerformanceChart}>
        <div class="top-ribbon">
          <span>Performance Chart</span>
          <button @click=${this.togglePerformanceChart}>Close</button>
        </div>
        <skottie-performance-sk id="chart"></skottie-performance-sk>
      </dialog>
    `;
  }

  private leftControls = () => {
    if (this.viewMode === 'presentation') {
      return null;
    }

    return html` <div class="json-chooser">
        <div class="title">JSON File</div>
        ${this.renderDownload()}
        <skottie-file-form-sk @files-selected=${this.skottieFilesSelected}></skottie-file-form-sk>
      </div>

      ${this.fileSettingsDialog()} ${this.slotManager()} ${this.colorManager()}
      ${this.backgroundDialog()} ${this.audioDialog()} ${this.optionsDialog()}`;
  };

  private rightControls = () => html`
    ${this.jsonTextEditor()} ${this.library()} ${this.embedDialog()}
  `;

  private renderDownload() {
    if (this.state.lottie) {
      return html`
        <div class="upload-download">
          <div class="large edit-config">
            ${this.state.filename} ${this.width}x${this.height} ...
          </div>
          <div class="download">
            <a target="_blank" download=${this.state.filename} href=${this.downloadURL}>
              <file-download-icon-sk></file-download-icon-sk>
            </a>
            ${!this.areChangesUploaded() ? '(without edits)' : ''}
          </div>
        </div>
      `;
    }
    return null;
  }

  private optionsDialog = () => html`
    <details class="expando">
      <summary id="options-open">
        <span>Options</span><expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>
      <div class="options-container">
        <checkbox-sk
          label="Show lottie-web"
          ?checked=${this.showLottie}
          @click=${this.toggleLottie}>
        </checkbox-sk>
        ${this.showLottie
          ? html`
              <skottie-dropdown-sk
                .name=${'lottie-renderer'}
                .options=${[
                  {
                    id: 'svg',
                    value: 'SVG',
                    selected: this.lottiePlayerRenderer === 'svg',
                  },
                  {
                    id: 'canvas',
                    value: 'Canvas',
                    selected: this.lottiePlayerRenderer === 'canvas',
                  },
                ]}
                @select=${this.onLottieRendererSelect}
                full>
              </skottie-dropdown-sk>
            `
          : ''}
      </div>
    </details>
  `;

  private audioDialog = () =>
    renderByDomain(
      html`
        <details
          class="expando"
          ?open=${this.showAudio}
          @toggle=${(e: Event) => this.toggleAudio((e.target! as HTMLDetailsElement).open)}>
          <summary id="audio-open">
            <span>Audio</span><expand-less-icon-sk></expand-less-icon-sk>
            <expand-more-icon-sk></expand-more-icon-sk>
          </summary>

          <skottie-audio-sk .animation=${this.state.lottie} @apply=${this.applyAudioSync}>
          </skottie-audio-sk>
        </details>
      `,
      AUDIO_SUPPORTED_DOMAINS
    );

  private fileSettingsDialog = () => html`
    <details
      class="expando"
      ?open=${this.showFileSettings}
      @toggle=${(e: Event) => this.toggleFileSettings((e.target! as HTMLDetailsElement).open)}>
      <summary id="fileSettings-open">
        <span>File Settings</span><expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>
      <skottie-file-settings-sk
        .width=${this.width}
        .height=${this.height}
        .fps=${this.fps}
        @settings-change=${this.skottieFileSettingsUpdated}></skottie-file-settings-sk>
    </details>
  `;

  private backgroundDialog = () => html`
    <details
      class="expando"
      ?open=${this.showBackgroundSettings}
      @toggle=${(e: Event) =>
        this.toggleBackgroundSettings((e.target! as HTMLDetailsElement).open)}>
      <summary>
        <span>Background color</span>
        <expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>
      <skottie-background-settings-sk
        @background-change=${this.skottieBackgroundUpdated}></skottie-background-settings-sk>
    </details>
  `;

  private colorManager = () => html`
    <details class="expando">
      <summary>
        <span>Color manager</span>
        <expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>
      <skottie-color-manager-sk
        .animation=${this.state.lottie}
        @animation-updated=${this.onColorManagerUpdated}></skottie-color-manager-sk>
    </details>
  `;

  private iframeDirections = () =>
    `<iframe width="${this.width}" height="${this.height}" src="${window.location.origin}/e/${this.hash}?w=${this.width}&h=${this.height}" scrolling=no>`;

  private inlineDirections = () =>
    `<skottie-inline-sk width="${this.width}" height="${this.height}" src="${window.location.origin}/_/j/${this.hash}"></skottie-inline-sk>`;

  private skottiePlayerTemplate = () =>
    html` <figure class="players-container-player">
      <skottie-player-sk paused width=${this.width} height=${this.height}> </skottie-player-sk>
      ${this.wasmCaption()}
    </figure>`;

  private lottiePlayerTemplate = () => {
    if (!this.showLottie) {
      return '';
    }
    return html` <figure class="players-container-player">
      <div
        id="container"
        title="lottie-web"
        style="width:${this.width}px;height:${this.height}px;background-color:${this
          .backgroundColor}"></div>
      ${caption('lottie-web', this.viewMode)}
    </figure>`;
  };

  private library = () =>
    html` <details
      class="expando"
      ?open=${this.showLibrary}
      @toggle=${(e: Event) => this.toggleLibrary((e.target! as HTMLDetailsElement).open)}>
      <summary id="library-open">
        <span>Library</span><expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>

      <skottie-library-sk @select=${this.updateAnimation}> </skottie-library-sk>
    </details>`;

  private jsonEditor = (): TemplateResult =>
    html` <dialog class="editor" ?open=${this.showJSONEditor}>
      <div class="top-ribbon">
        <span>Layer Information</span>
        <button @click=${this.toggleEditor}>Close</button>
      </div>
      <div id="json_editor"></div>
    </dialog>`;

  private gifExporter = () => html`
    <dialog class="export" ?open=${this.showGifExporter}>
      <div class="top-ribbon">
        <span>Export</span>
        <button @click=${this.toggleGifExporter}>Close</button>
      </div>
      <skottie-gif-exporter-sk @start=${this.onExportStart}> </skottie-gif-exporter-sk>
    </dialog>
  `;

  private jsonTextEditor = () => html`
    <skottie-text-editor-sk
      .animation=${this.state.lottie}
      .mode=${this.viewMode}
      @text-change=${this.onTextChange}>
    </skottie-text-editor-sk>
  `;

  private shaderEditor = () => html`
    <details
      class="expando"
      ?open=${this.showShaderEditor}
      @toggle=${(e: Event) => this.toggleShaderEditor((e.target! as HTMLDetailsElement).open)}>
      <summary>
        <span>Edit Shader</span><expand-less-icon-sk></expand-less-icon-sk>
        <expand-more-icon-sk></expand-more-icon-sk>
      </summary>

      <skottie-shader-editor-sk
        .animation=${this.state.lottie}
        .mode=${this.viewMode}
        @apply=${this.applyShaderEdits}>
      </skottie-shader-editor-sk>
    </details>
  `;

  private buildFileName = () => {
    const fileName = this.state.filename || this.state.lottie?.metadata?.filename;
    if (fileName) {
      return html`<div title="${fileName}">${fileName}</div>`;
    }
    return null;
  };

  private wasmCaption = () => {
    if (this.viewMode === 'presentation') {
      return null;
    }
    return html` <figcaption style="max-width: ${this.width}px;">
      <div>skottie-wasm</div>
      ${this.buildFileName()}
    </figcaption>`;
  };

  private assetsPath = PRODUCTION_ASSETS_PATH; // overridable for testing

  private backgroundColor: string = 'rgba(255,255,255,1)';

  // The URL referring to the lottie JSON Blob.
  private downloadURL: string = '';

  private duration: number = 0; // 0 is a sentinel value for "player not loaded yet"

  private editor: JSONEditor | null = null;

  private editorLoaded: boolean = false;

  // used for remembering the time elapsed while the animation is playing.
  private elapsedTime: number = 0;

  private fps: number = 0;

  private hash: string = '';

  private height: number = 0;

  private lottiePlayer: BodymovinPlayer | null = null;

  private lottiePlayerRenderer: RendererType = 'svg';

  private performanceChart: SkottiePerformanceSk | null = null;

  private playing: boolean = true;

  private playingOnStartOfScrub: boolean = false;

  // The wasm animation computes how long it has been since the previous rendered time and
  // uses arithmetic to figure out where to seek (i.e. which frame to draw).
  private previousFrameTime: number = 0;

  private scrubbing: boolean = false;

  private showAudio: boolean = false;

  private showCompatibilityReport: boolean = false;

  private showGifExporter: boolean = false;

  private showJSONEditor: boolean = false;

  private showLibrary: boolean = false;

  private showLottie: boolean = false;

  private showPerformanceChart: boolean = false;

  private showTextEditor: boolean = false;

  private showShaderEditor: boolean = false;

  private showFileSettings: boolean = false;

  private showBackgroundSettings: boolean = false;

  private skottieLibrary: SkottieLibrarySk | null = null;

  private skottiePlayer: SkottiePlayerSk | null = null;

  private speed: number = 1; // this is a playback multiplier

  private state: SkottieConfigState;

  private stateChanged: () => void;

  private ui: UIMode = 'idle';

  private viewMode: ViewMode = 'default';

  // This attribute will keep a reference to the tool that generated the last json change
  // It will be used to prevent reloading the tool if it's the one affecting the animation
  private changingTool: ToolType = 'none';

  private width: number = 0;

  private forceRedraw: boolean = false;

  constructor() {
    super(SkottieSk.template);

    this.state = {
      filename: '',
      lottie: null,
      assetsZip: '',
      assetsFilename: '',
    };

    this.stateChanged = stateReflector(
      /* getState */ () => ({
        // provide empty values
        l: this.showLottie,
        e: this.showJSONEditor,
        g: this.showGifExporter,
        t: this.showTextEditor,
        s: this.showShaderEditor,
        p: this.showPerformanceChart,
        c: this.showCompatibilityReport,
        i: this.showLibrary,
        a: this.showAudio,
        w: this.width,
        h: this.height,
        f: this.fps,
        bg: this.backgroundColor,
        mode: this.viewMode,
        fs: this.showFileSettings,
        b: this.showBackgroundSettings,
      }),
      /* setState */ (newState) => {
        this.showLottie = !!newState.l;
        this.showJSONEditor = !!newState.e;
        this.showGifExporter = !!newState.g;
        this.showTextEditor = !!newState.t;
        this.showShaderEditor = !!newState.s;
        this.showPerformanceChart = !!newState.p;
        this.showCompatibilityReport = !!newState.c;
        this.showLibrary = !!newState.i;
        this.showAudio = !!newState.a;
        this.width = +newState.w;
        this.height = +newState.h;
        this.fps = +newState.f;
        this.showFileSettings = !!newState.fs;
        this.showBackgroundSettings = !!newState.b;
        this.viewMode = newState.mode === 'presentation' ? 'presentation' : 'default';
        this.backgroundColor = String(newState.bg);
        this.render();
      }
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.reflectFromURL();
    window.addEventListener('popstate', this.reflectFromURL);
    this.render();

    // Start a continuous animation loop.
    const drawFrame = () => {
      window.requestAnimationFrame(drawFrame);

      // Elsewhere, the _previousFrameTime is set to null to restart
      // the animation. If null, we assume the user hit re-wind
      // and restart both the Skottie animation and the lottie-web one.
      // This avoids the (small) boot-up lag while we wait for the
      // skottie animation to be parsed and loaded.
      if (!this.previousFrameTime && this.playing) {
        this.previousFrameTime = Date.now();
        this.elapsedTime = 0;
      }
      if (this.playing && this.duration > 0) {
        const currentTime = Date.now();
        this.elapsedTime += (currentTime - this.previousFrameTime) * this.speed;
        this.previousFrameTime = currentTime;
        const progress = this.elapsedTime % this.duration;

        // If we want to have synchronized playing, it's best to force
        // all players to draw the same frame rather than letting them play
        // on their own timeline.
        const normalizedProgress = progress / this.duration;
        this.performanceChart?.start(progress, this.duration, this.state.lottie?.fr || 0);
        this.skottiePlayer?.seek(normalizedProgress, this.forceRedraw);
        this.performanceChart?.end();
        this.skottieLibrary?.seek(normalizedProgress);

        // lottie player takes the milliseconds from the beginning of the animation.
        this.lottiePlayer?.goToAndStop(progress);
        this.updateScrubber();
        this.updateFrameLabel();
      }
    };

    window.requestAnimationFrame(drawFrame);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener('popstate', this.reflectFromURL);
  }

  attributeChangedCallback(): void {
    this.render();
  }

  private updateAnimation(e: CustomEvent<LottieAnimation>): void {
    this.state.lottie = e.detail;
    this.state.filename = e.detail.metadata?.filename || this.state.filename;
    this.changingTool = 'skottie-library';
    this.ui = 'draft';
    this.render();
  }

  private onTextChange(ev: CustomEvent<TextEditEventDetail>): void {
    this.changingTool = 'text-edits';
    this.onAnimationUpdated(ev);
    this.loadAssetsAndRender();
  }

  private applyShaderEdits(e: CustomEvent<ShaderEditApplyEventDetail>): void {
    const shaders = e.detail.shaders;
    this.state.lottie = replaceShaders(shaders, this.state.lottie!);
    // TODO(jmbetancourt): support skottieLibrary
    // this.skottieLibrary?.replaceShaders(shaders);

    this.changingTool = 'shader-edits';
    this.ui = 'draft';
    this.render();
  }

  private applyAudioSync(e: CustomEvent<AudioStartEventDetail>): void {
    const detail = e.detail;
    this.speed = detail.speed;
    this.previousFrameTime = Date.now();
    this.elapsedTime = 0;
    if (!this.playing) {
      this.playpause();
    }
  }

  private onExportStart(): void {
    if (this.playing) {
      this.playpause();
    }
  }

  private applyEdits(): void {
    if (this.areChangesUploaded()) {
      return;
    }
    this.upload();
  }

  private autoSize(): boolean {
    let changed = false;
    if (!this.width) {
      this.width = this.state.lottie!.w;
      changed = true;
    }
    if (!this.height) {
      this.height = this.state.lottie!.h;
      changed = true;
    }
    // By default, leave FPS at 0, instead of reading them from the lottie,
    // because that will cause it to render as smoothly as possible,
    // which looks better in most cases. If a user gives a negative value
    // for fps (e.g. -1), then we use either what the lottie tells us or
    // as fast as possible.
    if (this.fps < 0) {
      this.fps = this.state.lottie!.fr || 0;
    }
    return changed;
  }

  private skottieFileSelected(e: CustomEvent<SkottieConfigEventDetail>) {
    this.state = e.detail.state;
    this.width = e.detail.width;
    this.height = e.detail.height;
    this.fps = e.detail.fps;
    this.backgroundColor = e.detail.backgroundColor;
    this.autoSize();
    this.stateChanged();
    if (e.detail.fileChanged) {
      this.upload();
    } else {
      this.ui = 'loaded';
      this.render();
      // Re-sync all players
      this.rewind();
    }
  }

  private skottieFileSettingsUpdated(e: CustomEvent<SkottieFileSettingsEventDetail>) {
    this.width = e.detail.width;
    this.height = e.detail.height;
    this.fps = e.detail.fps;
    this.stateChanged();
    if (this.state.lottie) {
      this.autoSize();
    }
    this.ui = 'loaded';
    this.render();
  }

  private skottieFilesSelected(e: CustomEvent<SkottieFilesEventDetail>) {
    const state = e.detail;
    const width = state.lottie?.w || this.width;
    const height = state.lottie?.h || this.height;
    const fileSettings = $$<SkottieFileSettingsSk>('#file-settings', this);
    if (fileSettings) {
      fileSettings.width = width;
      fileSettings.height = height;
    }
    this.width = width;
    this.height = height;
    this.state = state;
    this.upload();
  }

  private skottieBackgroundUpdated(e: CustomEvent<SkottieBackgroundSettingsEventDetail>) {
    const background = e.detail;
    this.backgroundColor = background.color;
    this.changingTool = 'background-color';
    this.ui = 'draft';
    this.stateChanged();
    if (this.state.lottie) {
      this.autoSize();
      // Re-sync all players
      this.rewind();
    }

    this.render();
  }

  private selectionCancelled() {
    this.ui = 'loaded';
    this.render();
  }

  private initializePlayer(): Promise<void> {
    if (!this.isToolUnsynced('skottie-player')) {
      return Promise.resolve();
    }
    return this.skottiePlayer!.initialize({
      width: this.width,
      height: this.height,
      lottie: this.state.lottie!,
      assets: this.state.assets,
      soundMap: this.state.soundMap,
      fps: this.fps,
    }).then(() => {
      this.performanceChart?.reset();
      this.duration = this.skottiePlayer!.duration();
      // If the user has specified a value for FPS, we want to lock the
      // size of the scrubber so it is as discrete as the frame rate.
      if (this.fps) {
        const scrubber = $$<HTMLInputElement>('#scrub', this);
        if (scrubber) {
          // calculate a scaled version of ms per frame as the step size.
          scrubber.step = String((1000 / this.fps) * (SCRUBBER_RANGE / this.duration));
        }
      }
      const frameTotal = $$<HTMLInputElement>('#frameTotal', this);
      if (frameTotal) {
        if (this.state.lottie!.fr) {
          frameTotal.textContent = `of ${String(
            Math.round(this.duration * (this.state.lottie!.fr / 1000))
          )}`;
        }
      }
      this.renderSlotManager();
    });
  }

  private fetchAdditionalAssets(): Promise<string[]> {
    if (!this.hash) {
      return Promise.resolve([]);
    }

    return fetch(`/_/r/${this.hash}`)
      .then(jsonOrThrow)
      .then((json) => {
        const allFileNames: string[] = json.files;
        const additionalAssets: string[] = [];
        if (allFileNames) {
          for (const fileName of allFileNames) {
            const ext: string | undefined = fileName.split('.').pop();
            if (ext && (ext === 'png' || ext === 'jpg')) {
              additionalAssets.push(fileName);
            }
          }
        }
        return additionalAssets;
      });
  }

  private loadAssetsAndRender(): Promise<void> {
    // We always fetch additional asset list before loading assets
    // While more readable, could be further optimized if startup time gets too bloated as a result
    return this.fetchAdditionalAssets().then((additionalAssets: string[]) => {
      const toLoad: Promise<LoadedAsset | null>[] = [];

      const lottie = this.state.lottie!;
      let fonts: FontAsset[] = [];
      let assets: LottieAsset[] = [];
      let loadAdditionalAssets: boolean = false;
      if (lottie.fonts && lottie.fonts.list) {
        fonts = lottie.fonts.list;
      }
      if (lottie.assets && lottie.assets.length) {
        assets = lottie.assets;

        // check for slot ids
        for (const asset of assets) {
          if (!isBinaryAsset(asset)) {
            continue;
          }
          if (asset.sid) {
            loadAdditionalAssets = true;
            break;
          }
        }
      }

      toLoad.push(...this.loadFonts(fonts));
      if (loadAdditionalAssets) {
        toLoad.push(...this.loadAssets(assets, additionalAssets));
      } else {
        toLoad.push(...this.loadAssets(assets, []));
      }
      return Promise.all(toLoad)
        .then((externalAssets: (LoadedAsset | null)[]) => {
          const loadedAssets: Record<string, ArrayBuffer> = {};
          const sounds = new SoundMap();
          for (const asset of externalAssets) {
            if (asset && asset.bytes) {
              loadedAssets[asset.name] = asset.bytes;
            } else if (asset && asset.player) {
              sounds.setPlayer(asset.name, asset.player);
            }
          }

          // check fonts
          fonts.forEach((font: FontAsset) => {
            if (!loadedAssets[font.fName]) {
              console.error(`Could not load font '${font.fName}'.`);
            }
          });

          this.state.assets = loadedAssets;
          this.state.soundMap = sounds;
          if (this.ui === 'synced') {
            this.ui = 'loaded';
          } else if (this.ui === 'unsynced') {
            this.ui = 'draft';
          }
          this.renderSlotManager();
          this.render();
        })
        .catch(() => {
          this.render();
        });
    });
  }

  private loadFonts(fonts: FontAsset[]): Promise<LoadedAsset | null>[] {
    const promises: Promise<LoadedAsset | null>[] = [];
    for (const font of fonts) {
      if (!font.fName) {
        continue;
      }

      const fetchFont = (fontURL: string) => {
        promises.push(
          fetch(fontURL).then((resp: Response) => {
            // fetch does not reject on 404
            if (!resp.ok) {
              return null;
            }
            return resp.arrayBuffer().then((buffer: ArrayBuffer) => ({
              name: font.fName,
              bytes: buffer,
            }));
          })
        );
      };

      const fontName = (name: string) => {
        // Normalize font names for the "Flex" variable font families (GoogleSansFlex, RobotoFlex).
        // This drops the style suffixes (e.g. "-ThinItalic").
        const regex = new RegExp('Flex-.*');
        return name.replace(regex, 'Flex');
      };

      // We have a mirror of google web fonts with a flattened directory structure which
      // makes them easier to find. Additionally, we can host the full .ttf
      // font, instead of the .woff2 font which is served by Google due to
      // it's smaller size by being a subset based on what glyphs are rendered.
      // Since we don't know all the glyphs we need up front, it's easiest
      // to just get the full font as a .ttf file.
      fetchFont(`${GOOGLE_WEB_FONTS_HOST}/${fontName(font.fName)}.ttf`);

      // Also try using uploaded assets.
      // We may end up with two different blobs for the same font name, in which case
      // the user-provided one takes precedence.
      if (this.hash) {
        fetchFont(`${this.assetsPath}/${this.hash}/${font.fName}.ttf`);
      }
    }
    return promises;
  }

  private loadAssets(
    assets: LottieAsset[],
    additionalAssets: string[]
  ): Promise<LoadedAsset | null>[] {
    const alreadyPromised = new Set<string>();
    const promises: Promise<LoadedAsset | null>[] = [];
    for (const asset of assets) {
      if (!isBinaryAsset(asset)) {
        continue;
      }
      if (asset.id.startsWith('audio_')) {
        // Howler handles our audio assets, they don't provide a promise when making a new Howl.
        // We push the audio asset as is and hope that it loads before playback starts.
        const inline = asset.p && asset.p.startsWith && asset.p.startsWith('data:');
        if (inline) {
          promises.push(
            Promise.resolve({
              name: asset.id,
              player: new AudioPlayer(asset.p),
            })
          );
        } else {
          promises.push(
            Promise.resolve({
              name: asset.id,
              player: new AudioPlayer(`${this.assetsPath}/${this.hash}/${asset.p}`),
            })
          );
        }
      } else {
        // asset.p is the filename, if it's an image.
        // Don't try to load inline/dataURI images.
        const should_load = asset.p && asset.p.startsWith && !asset.p.startsWith('data:');
        if (should_load) {
          alreadyPromised.add(asset.p);
          promises.push(
            fetch(`${this.assetsPath}/${this.hash}/${asset.p}`).then((resp: Response) => {
              // fetch does not reject on 404
              if (!resp.ok) {
                console.error(`Could not load ${asset.p}: status ${resp.status}`);
                return null;
              }
              return resp.arrayBuffer().then((buffer) => ({
                name: asset.p,
                bytes: buffer,
              }));
            })
          );
        }
      }
    }
    for (const assetName of additionalAssets) {
      if (!alreadyPromised.has(assetName)) {
        promises.push(
          fetch(`${this.assetsPath}/${this.hash}/${assetName}`).then((resp: Response) => {
            // fetch does not reject on 404
            if (!resp.ok) {
              console.error(`Could not load ${assetName}: status ${resp.status}`);
              return null;
            }
            return resp.arrayBuffer().then((buffer) => ({
              name: assetName,
              bytes: buffer,
            }));
          })
        );
      }
    }
    return promises;
  }

  private playpause(): void {
    const audioManager = $$<SkottieAudioSk>('skottie-audio-sk');
    if (this.playing) {
      this.lottiePlayer?.pause();
      this.state.soundMap?.pause();
      $$<HTMLElement>('#playpause-pause')!.style.display = 'none';
      $$<HTMLElement>('#playpause-play')!.style.display = 'inherit';
      audioManager?.pause();
    } else {
      this.lottiePlayer?.play();
      this.previousFrameTime = Date.now();
      // There is no need call a soundMap.play() function here.
      // Skottie invokes the play by calling seek on the needed audio track.
      $$<HTMLElement>('#playpause-pause')!.style.display = 'inherit';
      $$<HTMLElement>('#playpause-play')!.style.display = 'none';
      audioManager?.resume();
    }
    this.playing = !this.playing;
  }

  private recoverFromError(msg: string): void {
    errorMessage(msg);
    console.error(msg);
    window.history.pushState(null, '', '/');
    // For development we recover to the loaded state to see the animation
    // even if the upload didn't work
    this.ui = isDomain(SUPPORTED_DOMAINS.LOCALHOST) ? 'loaded' : 'idle';
    this.render();
  }

  private reflectFromURL(): void {
    // Check URL.
    const match = window.location.pathname.match(/\/([a-zA-Z0-9]+)/);
    if (!match) {
      this.hash = DEFAULT_LOTTIE_FILE;
    } else {
      this.hash = match[1];
    }
    this.ui = 'loading';
    this.render();
    // Run this on the next micro-task to allow mocks to be set up if needed.
    setTimeout(() => {
      fetch(`/_/j/${this.hash}`, {
        credentials: 'include',
      })
        .then(jsonOrThrow)
        .then((json) => {
          // remove legacy fields from state, if they are there.
          delete json.width;
          delete json.height;
          delete json.fps;
          this.state = json;

          if (this.autoSize()) {
            this.stateChanged();
          }
          this.ui = 'loaded';
          this.loadAssetsAndRender().then(() => {
            this.dispatchEvent(new CustomEvent('initial-animation-loaded', { bubbles: true }));
          });
        })
        .catch((msg) => this.recoverFromError(msg));
    });
  }

  private render(): void {
    if (this.downloadURL) {
      URL.revokeObjectURL(this.downloadURL);
    }
    this.downloadURL = URL.createObjectURL(new Blob([JSON.stringify(this.state.lottie)]));
    super._render();

    this.skottiePlayer = $$<SkottiePlayerSk>('skottie-player-sk', this);
    this.performanceChart = $$<SkottiePerformanceSk>('skottie-performance-sk', this);
    this.skottieLibrary = $$<SkottieLibrarySk>('skottie-library-sk', this);

    const skottieGifExporter = $$<SkottieGifExporterSk>('skottie-gif-exporter-sk', this);
    if (skottieGifExporter && this.skottiePlayer) {
      skottieGifExporter.player = this.skottiePlayer;
    }

    if (this.isPlayerView()) {
      if (this.state.soundMap && this.state.soundMap.map.size > 0) {
        this.hideVolumeSlider(false);
        // Stop any audio assets that start playing on frame 0
        // Pause the playback to force a user gesture to resume the AudioContext
        if (this.playing) {
          this.playpause();
          this.rewind();
        }
        this.state.soundMap.stop();
      } else {
        this.hideVolumeSlider(true);
      }
      try {
        this.initializePlayer().then(() => this.rewind());
        this.renderLottieWeb();
        this.renderJSONEditor();
        this.renderTextEditor();
        this.renderShaderEditor();
        this.renderAudioManager();
      } catch (e) {
        console.warn('caught error while rendering third party code', e);
      }
    }
    if (this.ui === 'draft') {
      this.ui = 'unsynced';
    } else if (this.ui === 'loaded') {
      this.ui = 'synced';
    }
    this.changingTool = 'none';
  }

  private renderAudioManager(): void {
    if (this.showAudio) {
      const audioManager = $$<SkottieAudioSk>('skottie-audio-sk', this);
      if (audioManager) {
        audioManager.animation = this.state.lottie!;
      }
    }
  }

  private renderTextEditor(): void {
    if (this.showTextEditor) {
      const textEditor = $$<SkottieTextEditorSk>('skottie-text-editor-sk', this);
      if (textEditor) {
        textEditor.animation = this.state.lottie!;
      }
    }
  }

  private renderSlotManager(): void {
    const slotManager = $$<SkottieSlotManagerSk>('skottie-slot-manager-sk', this);
    if (slotManager) {
      slotManager.player = this.skottiePlayer!;
      if (this.state.assets) {
        slotManager.resourceList = Object.keys(this.state.assets);
      }
      if (slotManager.hasSlots()) {
        this.forceRedraw = true;
      }
    }
  }

  private renderShaderEditor(): void {
    if (this.showShaderEditor) {
      const shaderEditor = $$<ShaderEditorSk>('skottie-shader-editor-sk', this);
      if (shaderEditor) {
        shaderEditor.animation = this.state.lottie!;
      }
    }
  }

  private renderJSONEditor(): void {
    if (!this.showJSONEditor) {
      this.editorLoaded = false;
      this.editor = null;
      return;
    }

    const editorContainer = $$<HTMLDivElement>('#json_editor')!;

    // See https://github.com/josdejong/svelte-jsoneditor/tree/main?tab=readme-ov-file#api
    // for documentation on this editor.
    const editorProps: JSONEditorPropsOptional = {
      onChange: () => {
        if (!this.editorLoaded) {
          return;
        }
        this.changingTool = 'json-editor';
        this.ui = 'draft';

        const lottie = toJSONContent(this.editor!.get()).json;
        this.state.lottie = lottie as LottieAnimation;
        this.render();
      },
    };

    editorProps.validator = createAjvValidator({
      // TODO(bwils) include feature schemas as well? More of UX problem
      schema: lottieSchema,
      onCreateAjv: () =>
        // Override ajv instance to support json schema 2020-12
        new Ajv({
          allErrors: true,
          verbose: true,
          strict: false,
        }),
    });

    const editorOptions = {
      target: editorContainer,
      props: editorProps,
    };

    // Only set the JSON when it is loaded, either because it's
    // the first time we got it from the server or because the user
    // made changes.
    if (!this.editor) {
      this.editorLoaded = false;
      editorContainer.innerHTML = '';
      this.editor = new JSONEditor(editorOptions);
      this.editor.set({ json: this.state.lottie });
    } else if (this.isToolUnsynced('json-editor')) {
      this.editorLoaded = false;
      this.editor.set({ json: this.state.lottie });
    }
    // We are now pretty confident that the onChange events will only be
    // when the user modifies the JSON.
    this.editorLoaded = true;
  }

  private renderLottieWeb(): void {
    if (!this.showLottie) {
      if (this.lottiePlayer) {
        this.lottiePlayer.destroy();
        this.lottiePlayer = null;
      }
      return;
    }
    // Don't re-start the animation while the user edits.
    if (this.isToolUnsynced('lottie-player') || !this.lottiePlayer) {
      if (this.lottiePlayer) {
        this.lottiePlayer.destroy();
      }
      $$<HTMLDivElement>('#container')!.innerHTML = '';
      this.lottiePlayer = LottiePlayer.loadAnimation({
        container: $$('#container')!,
        renderer: this.lottiePlayerRenderer,
        loop: true,
        autoplay: this.playing,
        assetsPath: `${this.assetsPath}/${this.hash}/`,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this.state.lottie)) as LottieAnimation,
        rendererSettings: {
          preserveAspectRatio: 'xMidYMid meet',
        },
      });
    }
  }

  // This fires every time the user moves the scrub slider.
  private onScrub(e: Event): void {
    if (!this.scrubbing) {
      // Pause the animation while dragging the slider.
      this.playingOnStartOfScrub = this.playing;
      if (this.playing) {
        this.playpause();
      }
      this.scrubbing = true;
    }
    const scrubber = (e.target as HTMLInputElement)!;
    const seek = +scrubber.value / SCRUBBER_RANGE;
    this.seek(seek);
    this.updateFrameLabel();
  }

  // This fires when the user releases the scrub slider.
  private onScrubEnd(): void {
    if (this.playingOnStartOfScrub) {
      this.playpause();
    }
    this.scrubbing = false;
  }

  private onFrameFocus(): void {
    if (this.playing) {
      this.playpause();
    }
  }

  private onFrameChange(): void {
    if (this.playing) {
      this.playpause();
    }
    const frameInput = $$<HTMLInputElement>('#frameInput', this);
    if (frameInput) {
      const frame = +frameInput.value;
      this.seekFrame(frame);
    }
  }

  private onChartClick(e: Event): void {
    const chart = $$<SkottiePerformanceSk>('#chart', this);
    const frame: number | undefined = chart?.getClickedFrame(e);
    if (frame !== undefined && frame !== -1) {
      if (this.playing) {
        this.playpause();
      }
      const frameInput = $$<HTMLInputElement>('#frameInput', this);
      if (frameInput) frameInput.value = String(frame);
      this.seekFrame(frame);
    }
  }

  private seekFrame(frame: number): void {
    if (frame > 0 && frame < this.duration) {
      let seek = 0;
      if (this.state.lottie?.fr) {
        seek = ((frame / this.state.lottie.fr) * 1000) / this.duration;
      }
      this.seek(seek);
      this.updateScrubber();
    }
  }

  private updateScrubber(): void {
    const scrubber = $$<HTMLInputElement>('#scrub', this);
    if (scrubber) {
      // Scale from time to the arbitrary scrubber range.
      const progress = this.elapsedTime % this.duration;
      scrubber.value = String((SCRUBBER_RANGE * progress) / this.duration);
    }
  }

  private updateFrameLabel(): void {
    const frameLabel = $$<HTMLInputElement>('#frameInput', this);
    if (frameLabel) {
      const progress = this.elapsedTime % this.duration;
      if (this.state.lottie!.fr) {
        frameLabel.value = String(Math.round(progress * (this.state.lottie!.fr / 1000)));
      }
    }
  }

  private seek(t: number): void {
    // catch case where t = 1
    t = Math.min(t, 0.9999);
    this.elapsedTime = t * this.duration;
    this.lottiePlayer?.goToAndStop(t * this.duration);
    this.skottiePlayer?.seek(t, this.forceRedraw);
    this.skottieLibrary?.seek(t);
  }

  private onVolumeChange(e: Event): void {
    const scrubber = (e.target as HTMLInputElement)!;
    this.state.soundMap?.setVolume(+scrubber.value);
  }

  private rewind(): void {
    // Handle rewinding when paused.
    if (!this.playing) {
      this.skottiePlayer!.seek(0, this.forceRedraw);
      this.skottieLibrary?.seek(0);
      this.previousFrameTime = 0;
      this.lottiePlayer?.goToAndStop(0);
      const scrubber = $$<HTMLInputElement>('#scrub', this);
      if (scrubber) {
        scrubber.value = '0';
      }
    } else {
      this.lottiePlayer?.goToAndPlay(0);
      this.previousFrameTime = 0;
      const audioManager = $$<SkottieAudioSk>('skottie-audio-sk', this);
      audioManager?.rewind();
    }
  }

  private toggleEditor(e: Event): void {
    // avoid double toggles
    e.preventDefault();

    const showJSONEditor = this.showJSONEditor;
    this.closeAllDialogs();
    this.showTextEditor = false;
    this.showJSONEditor = !showJSONEditor;
    this.stateChanged();
    this.render();
  }

  private toggleGifExporter(e: Event): void {
    // avoid double toggles
    e.preventDefault();
    this.showGifExporter = !this.showGifExporter;
    this.stateChanged();
    this.render();
  }

  private exportSelectHandler(e: CustomEvent<DropdownSelectEvent>): void {
    if (!this.skottiePlayer) {
      return;
    }
    const exportManager = $$<SkottieExporterSk>('skottie-exporter-sk');
    exportManager?.export(e.detail.value as ExportType, this.skottiePlayer);
  }

  private closeAllDialogs() {
    this.showPerformanceChart = false;
    this.showJSONEditor = false;
    this.showCompatibilityReport = false;
  }

  private togglePerformanceChart(e: Event): void {
    // avoid double toggles
    e.preventDefault();

    const showPerformanceChart = this.showPerformanceChart;
    this.closeAllDialogs();
    this.showPerformanceChart = !showPerformanceChart;
    this.stateChanged();
    this.render();
  }

  private toggleCompatibilityReport(e: Event): void {
    // avoid double toggles
    e.preventDefault();

    const showCompatibilityReport = this.showCompatibilityReport;
    this.closeAllDialogs();
    this.showCompatibilityReport = !showCompatibilityReport;
    this.stateChanged();
    this.render();
  }

  private toggleShaderEditor(open: boolean): void {
    this.showJSONEditor = false;
    this.showShaderEditor = open;
    this.stateChanged();
    this.render();
  }

  private toggleLibrary(open: boolean): void {
    this.showLibrary = open;
    this.stateChanged();
    this.render();
  }

  private toggleAudio(open: boolean): void {
    this.showAudio = open;
    this.stateChanged();
    this.render();
  }

  private toggleFileSettings(open: boolean): void {
    this.showFileSettings = open;
    this.stateChanged();
    this.render();
  }

  private toggleBackgroundSettings(open: boolean): void {
    this.showBackgroundSettings = open;
    this.stateChanged();
    this.render();
  }

  private toggleLottie(e: Event): void {
    // avoid double toggles
    e.preventDefault();
    this.showLottie = !this.showLottie;
    this.stateChanged();
    this.render();
  }

  private hideVolumeSlider(v: boolean) {
    const collapse = $$<CollapseSk>('#volume', this);
    if (collapse) {
      collapse.closed = v;
    }
  }

  private upload(): void {
    // POST the JSON to /_/upload
    this.hash = '';
    this.ui = 'draft';
    this.editorLoaded = false;
    this.editor = null;
    // Clean up the old animation and other wasm objects
    this.render();
    fetch('/_/upload', {
      credentials: 'include',
      body: JSON.stringify(this.state),
      headers: {
        'Content-Type': 'application/json',
      },
      method: 'POST',
    })
      .then(jsonOrThrow)
      .then((json) => {
        // Should return with the hash and the lottie file
        this.ui = 'loaded';
        this.hash = json.hash;
        this.state.lottie = json.lottie;
        window.history.pushState(null, '', `/${this.hash}`);
        this.stateChanged();
        if (this.state.assetsZip) {
          this.loadAssetsAndRender();
        }
        this.render();
      })
      .catch((msg) => this.recoverFromError(msg));

    if (!this.state.assetsZip) {
      this.ui = 'loaded';
      // Start drawing right away, no need to wait for
      // the JSON to make a round-trip to the server, since there
      // are no assets that we need to unzip server-side.
      // We still need to check for things like webfonts.
      this.render();
      this.loadAssetsAndRender();
    } else {
      // We have to wait for the server to process the zip file.
      this.ui = 'loading';
      this.render();
    }
  }

  private onLottieRendererSelect(ev: CustomEvent<DropdownSelectEvent>) {
    this.lottiePlayerRenderer = ev.detail.value as RendererType;

    // Re-initialize Lottie.
    this.lottiePlayer!.destroy();
    this.lottiePlayer = null;
    this.render();
  }

  private onColorManagerUpdated(ev: CustomEvent<SkottieTemplateEventDetail>) {
    this.changingTool = 'color-manager';
    this.onAnimationUpdated(ev);
  }

  private onSlotManagerUpdated(ev: CustomEvent<SkottieTemplateEventDetail>) {
    this.changingTool = 'slot-manager';
    this.onAnimationUpdated(ev);
  }

  private onAnimationUpdated(
    ev: CustomEvent<SkottieTemplateEventDetail | TextEditEventDetail>
  ): void {
    this.state.lottie = ev.detail.animation;
    this.ui = 'draft';
    this.render();
  }

  overrideAssetsPathForTesting(p: string): void {
    this.assetsPath = p;
  }

  private isToolUnsynced(tool: ToolType): boolean {
    return ['draft', 'loaded'].includes(this.ui) && this.changingTool !== tool;
  }

  private areChangesUploaded(): boolean {
    return !['draft', 'unsynced'].includes(this.ui);
  }

  private isPlayerView(): boolean {
    return ['draft', 'unsynced', 'synced', 'loaded'].includes(this.ui);
  }
}

define('skottie-sk', SkottieSk);
