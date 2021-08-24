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
import 'elements-sk/checkbox-sk';
import 'elements-sk/collapse-sk';
import 'elements-sk/error-toast-sk';
import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import JSONEditor, { JSONEditorOptions } from 'jsoneditor';
import { CollapseSk } from 'elements-sk/collapse-sk/collapse-sk';
import { SkottieGifExporterSk } from '../skottie-gif-exporter-sk/skottie-gif-exporter-sk';
import '../skottie-gif-exporter-sk';
import '../skottie-text-editor';
import { replaceTexts } from '../skottie-text-editor/text-replace';
import '../skottie-library-sk';
import { SoundMap, AudioPlayer } from '../audio';
import '../skottie-performance-sk';
import { renderByDomain } from '../helpers/templates';
import { supportedDomains } from '../helpers/domains';
import '../skottie-audio-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SkottieConfigEventDetail, SkottieConfigState } from '../skottie-config-sk/skottie-config-sk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';
import { SkottiePerformanceSk } from '../skottie-performance-sk/skottie-performance-sk';
import {
  FontAsset, LottieAnimation, LottieAsset, ViewMode,
} from '../types';
import { SkottieLibrarySk } from '../skottie-library-sk/skottie-library-sk';
import { AudioStartEventDetail, SkottieAudioSk } from '../skottie-audio-sk/skottie-audio-sk';

import { SKIA_VERSION } from '../../build/version';
import { SkottieTextEditorSk, TextEditApplyEventDetail } from '../skottie-text-editor/skottie-text-editor';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const JSONEditorConstructor: new(e: HTMLElement, o: JSONEditorOptions)=> JSONEditor = require('jsoneditor/dist/jsoneditor-minimalist.js');

interface BodymovinPlayer {
  goToAndStop(t: number): void;
  goToAndPlay(t: number): void;
  pause(): void;
  play(): void;
}

interface LottieLibrary {
  version: string;
  loadAnimation(opts: Record<string, unknown>): BodymovinPlayer;
}

interface LoadedAsset {
  name: string;
  bytes?: ArrayBuffer;
  player?: AudioPlayer;
}

// eslint-disable-next-line @typescript-eslint/no-var-requires
const bodymovin: LottieLibrary = require('lottie-web/build/player/lottie.min.js');

const GOOGLE_WEB_FONTS_HOST = 'https://storage.googleapis.com/skia-cdn/google-web-fonts';

const PRODUCTION_ASSETS_PATH = '/_/a';

// Make this the hash of the lottie file you want to play on startup.
const DEFAULT_LOTTIE_FILE = '1112d01d28a776d777cebcd0632da15b'; // gear.json

// SCRUBBER_RANGE is the input range for the scrubbing control.
// This is an arbitrary value, and is treated as a re-scaled duration.
const SCRUBBER_RANGE = 1000;

const AUDIO_SUPPORTED_DOMAINS = [
  supportedDomains.SKOTTIE_INTERNAL,
  supportedDomains.SKOTTIE_TENOR,
  supportedDomains.LOCALHOST,
];

type UIMode = 'dialog' | 'loading' | 'loaded';

const caption = (text: string, mode: ViewMode) => {
  if (mode === 'presentation') {
    return null;
  }
  return html`
  <figcaption>
  ${text}
  </figcaption>
  `;
};

const performanceChart = (show: boolean) => {
  if (!show) {
    return '';
  }
  return html`
<skottie-performance-sk></skottie-performance-sk>`;
};

const redir = () => renderByDomain(
  html`
  <div>
    Googlers should use <a href="https://skottie-internal.skia.org">skottie-internal.skia.org</a>.
  </div>`,
  Object.values(supportedDomains).filter((domain: string) => domain !== supportedDomains.SKOTTIE_INTERNAL),
);

const displayLoading = () => html`
  <div class=loading>
    <spinner-sk active></spinner-sk><span>Loading...</span>
  </div>
`;

export class SkottieSk extends ElementSk {
  private static template = (ele: SkottieSk) => html`
<header>
  <h2>Skottie</h2>
  <span>
    <a href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'>
      ${SKIA_VERSION.slice(0, 7)}
    </a>
  </span>
</header>
<main>
  ${ele.pick()}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
  ${redir()}
</footer>
`;

  // pick the right part of the UI to display based on ele._ui.
  private pick = () => {
    switch (this.ui) {
      default:
      case 'dialog':
        return this.displayDialog();
      case 'loading':
        return displayLoading();
      case 'loaded':
        return this.displayLoaded();
    }
  };

  private displayDialog = () => html`
<skottie-config-sk .state=${this.state} .width=${this.width}
    .height=${this.height} .fps=${this.fps} .backgroundColor=${this.backgroundColor}
    @skottie-selected=${this.skottieFileSelected} @cancelled=${this.selectionCancelled}></skottie-config-sk>
`;

  private displayLoaded = () => html`
${this.controls()}
<collapse-sk id=embed closed>
  <p>
    <label>
      Embed using an iframe: <input size=120 value=${this.iframeDirections()}>
    </label>
  </p>
  <p>
    <label>
      Embed on skia.org: <input size=140 value=${this.inlineDirections()}>
    </label>
  </p>
</collapse-sk>
<section class=figures>
  <figure>
    ${this.skottiePlayerTemplate()}
  </figure>
  ${this.lottiePlayerTemplate()}
  ${this.audio()}
  ${this.library()}
  ${this.livePreview()}
</section>

${performanceChart(this.showPerformanceChart)}
${this.jsonEditor()}
${this.gifExporter()}
${this.jsonTextEditor()}
`;

  private controls = () => {
    if (this.viewMode === 'presentation') {
      return null;
    } return html`
  <button class=edit-config @click=${this.startEdit}>
  ${this.state.filename} ${this.width}x${this.height} ...
  </button>
  <div class=controls>
    <button id=rewind @click=${this.rewind}>Rewind</button>
    <button id=playpause @click=${this.playpause}>Pause</button>
    <button ?hidden=${!this.hasEdits} @click=${this.applyEdits}>Apply Edits</button>
    <div class=download>
      <a target=_blank download=${this.state.filename} href=${this.downloadURL}>
        JSON
      </a>
      ${this.hasEdits ? '(without edits)' : ''}
    </div>
    <checkbox-sk label="Show lottie-web"
                ?checked=${this.showLottie}
                @click=${this.toggleLottie}>
    </checkbox-sk>
    <checkbox-sk label="Show editor"
                ?checked=${this.showJSONEditor}
                @click=${this.toggleEditor}>
    </checkbox-sk>
    <checkbox-sk label="Show gif exporter"
                ?checked=${this.showGifExporter}
                @click=${this.toggleGifExporter}>
    </checkbox-sk>
    <checkbox-sk label="Show text editor"
                ?checked=${this.showTextEditor}
                @click=${this.toggleTextEditor}>
    </checkbox-sk>
    <checkbox-sk label="Show performance chart"
                ?checked=${this.showPerformanceChart}
                @click=${this.togglePerformanceChart}>
    </checkbox-sk>
    <checkbox-sk label="Show library"
                ?checked=${this.showLibrary}
                @click=${this.toggleLibrary}>
    </checkbox-sk>
    ${this.audioButton()}
    <button id=embed-btn @click=${this.toggleEmbed}>Embed</button>
    <div class=scrub>
      <input id=scrub type=range min=0 max=${SCRUBBER_RANGE + 1} step=0.1
          @input=${this.onScrub} @change=${this.onScrubEnd}>
    </div>
  </div>
  <collapse-sk id=volume closed>
    <p>
      Volume:
    </p>
    <input id=volume-slider type=range min=0 max=1 step=.05 value=1
      @input=${this.onVolumeChange}>
  </collapse-sk>
  `;
  };

  private audioButton = () => renderByDomain(
    html`<checkbox-sk label="Show audio"
     ?checked=${this.showAudio}
     @click=${this.toggleAudio}>
  </checkbox-sk>`,
    AUDIO_SUPPORTED_DOMAINS,
  );

  private iframeDirections = () => `<iframe width="${this.width}" height="${this.height}" src="${window.location.origin}/e/${this.hash}?w=${this.width}&h=${this.height}" scrolling=no>`;

  private inlineDirections = () => `<skottie-inline-sk width="${this.width}" height="${this.height}" src="${window.location.origin}/_/j/${this.hash}"></skottie-inline-sk>`;

  private skottiePlayerTemplate = () => html`
<skottie-player-sk paused width=${this.width} height=${this.height}>
</skottie-player-sk>
${this.wasmCaption()}`;

  private lottiePlayerTemplate = () => {
    if (!this.showLottie) {
      return '';
    }
    return html`
<figure>
  <div id=container title=lottie-web
       style='width: ${this.width}px; height: ${this.height}px; background-color: ${this.backgroundColor}'></div>
       ${caption(`lottie-web ${bodymovin.version}`, this.viewMode)}
</figure>`;
  };

  private audio = () => {
    if (!this.showAudio) {
      return '';
    }
    return renderByDomain(
      html`
    <section class=audio>
      <skottie-audio-sk
        .animation=${this.state.lottie}
        @apply=${this.applyAudioSync}
      >
      </skottie-audio-sk>
    </section>`,
      AUDIO_SUPPORTED_DOMAINS,
    );
  };

  private library = () => {
    if (!this.showLibrary) {
      return '';
    }
    return html`
<section class=library>
  <skottie-library-sk
    @select=${this.updateAnimation}
  >
  </skottie-library-sk>
</section>`;
  };

  // TODO(kjlubick): Make the live preview use skottie
  private livePreview = () => {
    if (!this.hasEdits || !this.showLottie) {
      return '';
    }
    if (this.hasEdits) {
      return html`
<figure>
  <div id=live title=live-preview
       style='width: ${this.width}px; height: ${this.height}px'></div>
  <figcaption>Preview [lottie-web]</figcaption>
</figure>`;
    }
    return '';
  };

  private jsonEditor = () => {
    if (!this.showJSONEditor) {
      return '';
    }
    return html`
<section class=editor>
  <div id=json_editor></div>
</section>`;
  };

  private gifExporter = () => {
    if (!this.showGifExporter) {
      return '';
    }
    return html`
<section class=editor>
  <skottie-gif-exporter-sk
    @start=${this.onGifExportStart}
  >
  </skottie-gif-exporter-sk>
</section>`;
  };

  private jsonTextEditor = () => {
    if (!this.showTextEditor) {
      return '';
    }
    return html`
<section class=editor>
  <skottie-text-editor
    .animation=${this.state.lottie}
    .mode=${this.viewMode}
    @apply=${this.applyTextEdits}
  >
  </skottie-text-editor>
</section>`;
  };

  private filename = () => {
    const lottie = this.state.lottie;
    if (lottie && lottie.metadata && lottie.metadata.filename) {
      return html`<div title='${lottie.metadata.filename}'>${lottie.metadata.filename}</div>`;
    }
    return null;
  };

  private wasmCaption = () => {
    if (this.viewMode === 'presentation') {
      return null;
    }
    return html`
  <figcaption style='max-width: ${this.width}px;'>
    <div>skottie-wasm</div>
    ${this.filename()}
  </figcaption>`;
  };

  private assetsPath = PRODUCTION_ASSETS_PATH; // overridable for testing

  // The URL referring to the lottie JSON Blob.
  private backgroundColor: string = 'rgba(0,0,0,0)';

  private downloadURL: string = '';

  private duration: number = 0; // 0 is a sentinel value for "player not loaded yet"

  private editor: JSONEditor | null = null;

  private editorLoaded: boolean = false;

  // used for remembering the time elapsed while the animation is playing.
  private elapsedTime: number = 0;

  private fps: number = 0;

  private hasEdits: boolean = false;

  private hash: string = '';

  private height: number = 0;

  private live: BodymovinPlayer | null = null;

  private lottiePlayer: BodymovinPlayer | null = null;

  private performanceChart: SkottiePerformanceSk | null = null;

  private playing: boolean = true;

  private playingOnStartOfScrub: boolean = false;

  // The wasm animation computes how long it has been since the previous rendered time and
  // uses arithmetic to figure out where to seek (i.e. which frame to draw).
  private previousFrameTime: number = 0;

  private scrubbing: boolean = false;

  private showAudio: boolean = false;

  private showGifExporter: boolean = false;

  private showJSONEditor: boolean = false;

  private showLibrary: boolean = false;

  private showLottie: boolean = false;

  private showPerformanceChart: boolean = false;

  private showTextEditor: boolean = false;

  private skottieLibrary: SkottieLibrarySk | null = null;

  private skottiePlayer: SkottiePlayerSk | null = null;

  private speed: number = 1; // this is a playback multiplier

  private state: SkottieConfigState;

  private stateChanged: ()=> void;

  private ui: UIMode = 'dialog';

  private viewMode: ViewMode = 'default';

  private width: number = 0;

  constructor() {
    super(SkottieSk.template);

    this.state = {
      filename: '',
      lottie: null,
      assetsZip: '',
      assetsFilename: '',
    };

    this.stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        l: this.showLottie,
        e: this.showJSONEditor,
        g: this.showGifExporter,
        t: this.showTextEditor,
        p: this.showPerformanceChart,
        i: this.showLibrary,
        a: this.showAudio,
        w: this.width,
        h: this.height,
        f: this.fps,
        bg: this.backgroundColor,
        mode: this.viewMode,
      }), /* setState */(newState) => {
        this.showLottie = !!newState.l;
        this.showJSONEditor = !!newState.e;
        this.showGifExporter = !!newState.g;
        this.showTextEditor = !!newState.t;
        this.showPerformanceChart = !!newState.p;
        this.showLibrary = !!newState.i;
        this.showAudio = !!newState.a;
        this.width = +newState.w;
        this.height = +newState.h;
        this.fps = +newState.f;
        this.viewMode = newState.mode === 'presentation' ? 'presentation' : 'default';
        this.backgroundColor = String(newState.bg);
        this.render();
      },
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
        this.skottiePlayer?.seek(normalizedProgress);
        this.performanceChart?.end();
        this.skottieLibrary?.seek(normalizedProgress);

        // lottie player takes the milliseconds from the beginning of the animation.
        this.lottiePlayer?.goToAndStop(progress);
        this.live?.goToAndStop(progress);
        const scrubber = $$<HTMLInputElement>('#scrub', this);
        if (scrubber) {
          // Scale from time to the arbitrary scrubber range.
          scrubber.value = String((SCRUBBER_RANGE * progress) / this.duration);
        }
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
    this.upload();
  }

  private applyTextEdits(e: CustomEvent<TextEditApplyEventDetail>): void {
    const texts = e.detail.texts;
    this.state.lottie = replaceTexts(texts, this.state.lottie!);
    this.skottieLibrary?.replaceTexts(texts);

    this.upload();
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

  private onGifExportStart(): void {
    if (this.playing) {
      this.playpause();
    }
  }

  private applyEdits(): void {
    if (!this.editor || !this.editorLoaded || !this.hasEdits) {
      return;
    }
    this.state.lottie = this.editor.get();
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
      this.initializePlayer();
      // Re-sync all players
      this.rewind();
    }
  }

  private selectionCancelled() {
    this.ui = 'loaded';
    this.render();
    this.initializePlayer();
  }

  private initializePlayer(): Promise<void> {
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
    });
  }

  private loadAssetsAndRender(): Promise<void> {
    const toLoad: Promise<(LoadedAsset | null)>[] = [];

    const lottie = this.state.lottie!;
    let fonts: FontAsset[] = [];
    let assets: LottieAsset[] = [];
    if (lottie.fonts && lottie.fonts.list) {
      fonts = lottie.fonts.list;
    }
    if (lottie.assets && lottie.assets.length) {
      assets = lottie.assets;
    }

    toLoad.push(...this.loadFonts(fonts));
    toLoad.push(...this.loadAssets(assets));

    return Promise.all(toLoad).then((externalAssets: (LoadedAsset | null)[]) => {
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
      this.render();
      return this.initializePlayer().then(() => {
        // Re-sync all players
        this.rewind();
      });
    })
      .catch(() => {
        this.render();
        return this.initializePlayer().then(() => {
          // Re-sync all players
          this.rewind();
        });
      });
  }

  private loadFonts(fonts: FontAsset[]): Promise<LoadedAsset | null>[] {
    const promises: (Promise<LoadedAsset | null>)[] = [];
    for (const font of fonts) {
      if (!font.fName) {
        continue;
      }

      const fetchFont = (fontURL: string) => {
        promises.push(fetch(fontURL)
          .then((resp: Response) => {
            // fetch does not reject on 404
            if (!resp.ok) {
              return null;
            }
            return resp.arrayBuffer().then((buffer: ArrayBuffer) => ({
              name: font.fName,
              bytes: buffer,
            }));
          }));
      };

      // We have a mirror of google web fonts with a flattened directory structure which
      // makes them easier to find. Additionally, we can host the full .ttf
      // font, instead of the .woff2 font which is served by Google due to
      // it's smaller size by being a subset based on what glyphs are rendered.
      // Since we don't know all the glyphs we need up front, it's easiest
      // to just get the full font as a .ttf file.
      fetchFont(`${GOOGLE_WEB_FONTS_HOST}/${font.fName}.ttf`);

      // Also try using uploaded assets.
      // We may end up with two different blobs for the same font name, in which case
      // the user-provided one takes precedence.
      fetchFont(`${this.assetsPath}/${this.hash}/${font.fName}.ttf`);
    }
    return promises;
  }

  private loadAssets(assets: LottieAsset[]): (Promise<LoadedAsset | null>)[] {
    const promises: (Promise<LoadedAsset | null>)[] = [];
    for (const asset of assets) {
      if (asset.id.startsWith('audio_')) {
        // Howler handles our audio assets, they don't provide a promise when making a new Howl.
        // We push the audio asset as is and hope that it loads before playback starts.
        const inline = asset.p && asset.p.startsWith && asset.p.startsWith('data:');
        if (inline) {
          promises.push(Promise.resolve({
            name: asset.id,
            player: new AudioPlayer(asset.p),
          }));
        } else {
          promises.push(Promise.resolve({
            name: asset.id,
            player: new AudioPlayer(`${this.assetsPath}/${this.hash}/${asset.p}`),
          }));
        }
      } else {
        // asset.p is the filename, if it's an image.
        // Don't try to load inline/dataURI images.
        const should_load = asset.p && asset.p.startsWith && !asset.p.startsWith('data:');
        if (should_load) {
          promises.push(fetch(`${this.assetsPath}/${this.hash}/${asset.p}`)
            .then((resp: Response) => {
              // fetch does not reject on 404
              if (!resp.ok) {
                console.error(`Could not load ${asset.p}: status ${resp.status}`);
                return null;
              }
              return resp.arrayBuffer().then((buffer) => ({
                name: asset.p,
                bytes: buffer,
              }));
            }));
        }
      }
    }
    return promises;
  }

  private playpause(): void {
    const audioManager = $$<SkottieAudioSk>('skottie-audio-sk');
    if (this.playing) {
      this.lottiePlayer?.pause();
      this.live?.pause();
      this.state.soundMap?.pause();
      $$('#playpause')!.textContent = 'Play';
      audioManager?.pause();
    } else {
      this.lottiePlayer?.play();
      this.live?.play();
      this.previousFrameTime = Date.now();
      // There is no need call a soundMap.play() function here.
      // Skottie invokes the play by calling seek on the needed audio track.
      $$('#playpause')!.textContent = 'Pause';
      audioManager?.resume();
    }
    this.playing = !this.playing;
  }

  private recoverFromError(msg: string): void {
    errorMessage(msg);
    console.error(msg);
    window.history.pushState(null, '', '/');
    this.ui = 'dialog';
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
      }).then(jsonOrThrow).then((json) => {
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
          console.log('loaded');
          this.dispatchEvent(new CustomEvent('initial-animation-loaded', { bubbles: true }));
        });
      }).catch((msg) => this.recoverFromError(msg));
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

    if (this.ui === 'loaded') {
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
        this.renderLottieWeb();
        this.renderJSONEditor();
        this.renderTextEditor();
        this.renderAudioManager();
      } catch (e) {
        console.warn('caught error while rendering third party code', e);
      }
    }
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
      const textEditor = $$<SkottieTextEditorSk>('skottie-text-editor', this);
      if (textEditor) {
        textEditor.animation = this.state.lottie!;
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
    // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
    // for documentation on this editor.
    const editorOptions = {
      // Use original key order (this preserves related fields locality).
      sortObjectKeys: false,
      // There are sometimes a few onChange events that happen
      // during the initial .set(), so we have a safety variable
      // _editorLoaded to prevent a bunch of recursion
      onChange: () => {
        if (!this.editorLoaded) {
          return;
        }
        this.hasEdits = true;
        this.render();
      },
    };

    if (!this.editor) {
      this.editorLoaded = false;
      editorContainer.innerHTML = '';
      this.editor = new JSONEditorConstructor(editorContainer, editorOptions);
    }
    if (!this.hasEdits) {
      this.editorLoaded = false;
      // Only set the JSON when it is loaded, either because it's
      // the first time we got it from the server or because the user
      // hit applyEdits.
      this.editor.set(this.state.lottie);
    }
    // We are now pretty confident that the onChange events will only be
    // when the user modifies the JSON.
    this.editorLoaded = true;
  }

  private renderLottieWeb(): void {
    if (!this.showLottie) {
      return;
    }
    // Don't re-start the animation while the user edits.
    if (!this.hasEdits) {
      $$<HTMLDivElement>('#container')!.innerHTML = '';
      this.lottiePlayer = bodymovin.loadAnimation({
        container: $$('#container'),
        renderer: 'svg',
        loop: true,
        autoplay: this.playing,
        assetsPath: `${this.assetsPath}/${this.hash}/`,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this.state.lottie)),
        rendererSettings: {
          preserveAspectRatio: 'xMidYMid meet',
        },
      });
      this.live = null;
    } else {
      // we have edits, update the live preview version.
      // It will re-start from the very beginning, but the user can
      // hit "rewind" to re-sync them.
      $$<HTMLDivElement>('#live')!.innerHTML = '';
      this.live = bodymovin.loadAnimation({
        container: $$('#live'),
        renderer: 'svg',
        loop: true,
        autoplay: this.playing,
        assetsPath: `${this.assetsPath}/${this.hash}/`,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this.editor!.get())),
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
    const seek = (+scrubber.value / SCRUBBER_RANGE);
    this.elapsedTime = seek * this.duration;
    this.live?.goToAndStop(seek);
    this.lottiePlayer?.goToAndStop(seek * this.duration);
    this.skottiePlayer?.seek(seek);
    this.skottieLibrary?.seek(seek);
  }

  // This fires when the user releases the scrub slider.
  private onScrubEnd(): void {
    if (this.playingOnStartOfScrub) {
      this.playpause();
    }
    this.scrubbing = false;
  }

  private onVolumeChange(e: Event): void {
    const scrubber = (e.target as HTMLInputElement)!;
    this.state.soundMap?.setVolume(+scrubber.value);
  }

  private rewind(): void {
    // Handle rewinding when paused.
    if (!this.playing) {
      this.skottiePlayer!.seek(0);
      this.skottieLibrary?.seek(0);
      this.previousFrameTime = 0;
      this.live?.goToAndStop(0);
      this.lottiePlayer?.goToAndStop(0);
      const scrubber = $$<HTMLInputElement>('#scrub', this);
      if (scrubber) {
        scrubber.value = '0';
      }
    } else {
      this.live?.goToAndPlay(0);
      this.lottiePlayer?.goToAndPlay(0);
      this.previousFrameTime = 0;
      const audioManager = $$<SkottieAudioSk>('skottie-audio-sk', this);
      audioManager?.rewind();
    }
  }

  private startEdit(): void {
    this.ui = 'dialog';
    this.render();
  }

  private toggleEditor(e: Event): void {
    // avoid double toggles
    e.preventDefault();
    this.showTextEditor = false;
    this.showJSONEditor = !this.showJSONEditor;
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

  private togglePerformanceChart(e: Event): void {
    // avoid double toggles
    e.preventDefault();
    this.showPerformanceChart = !this.showPerformanceChart;
    this.stateChanged();
    this.render();
  }

  private toggleTextEditor(e: Event): void {
    e.preventDefault();
    this.showJSONEditor = false;
    this.showTextEditor = !this.showTextEditor;
    this.stateChanged();
    this.render();
  }

  private toggleLibrary(e: Event): void {
    e.preventDefault();
    this.showLibrary = !this.showLibrary;
    this.stateChanged();
    this.render();
  }

  private toggleAudio(e: Event): void {
    e.preventDefault();
    this.showAudio = !this.showAudio;
    this.stateChanged();
    this.render();
  }

  private toggleEmbed(): void {
    const collapse = $$<CollapseSk>('#embed', this);
    if (collapse) {
      collapse.closed = !collapse.closed;
    }
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
    this.hasEdits = false;
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
    }).then(jsonOrThrow).then((json) => {
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
    }).catch((msg) => this.recoverFromError(msg));

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

  overrideAssetsPathForTesting(p: string): void {
    this.assetsPath = p;
  }
}

define('skottie-sk', SkottieSk);
