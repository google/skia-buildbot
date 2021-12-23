/**
 * @module particles-sk
 * @description <h2><code>particles-sk</code></h2>
 *
 * <p>
 *   The main application element for particles.
 * </p>
 *
 * @attr paused - This attribute is only checked on the connectedCallback
 *   and is used to stop the player from starting the animation. This is
 *   only used for tests.
 *
 */
import '../particles-player-sk';
import '../particles-config-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import JSONEditor, { JSONEditorOptions } from 'jsoneditor';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParticlesConfig, ParticlesConfigSk } from '../particles-config-sk/particles-config-sk';
import { ParticlesPlayerSk } from '../particles-player-sk/particles-player-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ScrapBody, ScrapID } from '../json';

// It is assumed that this symbol is being provided by a version.js file loaded in before this
// file.
declare const SKIA_VERSION: string;

const defaultParticleDemo = {
  MaxCount: 1000,
  Drawable: {
    Type: 'SkCircleDrawable',
    Radius: 2,
  },
  Code: [
    'void effectSpawn(inout Effect effect) {',
    '  effect.rate = 200;',
    '  effect.color = float4(1, 0, 0, 1);',
    '}',
    '',
    'void spawn(inout Particle p) {',
    '  p.lifetime = 3 + rand(p.seed);',
    '  p.vel.y = -50;',
    '}',
    '',
    'void update(inout Particle p) {',
    '  float w = mix(15, 3, p.age);',
    '  p.pos.x = sin(radians(p.age * 320)) * mix(25, 10, p.age) + mix(-w, w, rand(p.seed));',
    '  if (rand(p.seed) < 0.5) { p.pos.x = -p.pos.x; }',
    '',
    '  p.color.g = (mix(75, 220, p.age) + mix(-30, 30, rand(p.seed))) / 255;',
    '}',
    '',
  ],
  Bindings: [],
};

const DEFAULT_SIZE = 800;

// State is the state that's reflected to the URL.
interface State {
  showEditor: boolean;
  width: number;
  height: number;
  nameOrHash: string;
}

const defaultState: State = {
  showEditor: true,
  width: DEFAULT_SIZE,
  height: DEFAULT_SIZE,
  nameOrHash: '',
};

type stateChangedCallback = ()=> void;

export class ParticlesSk extends ElementSk {
  private state: State = Object.assign({}, defaultState)

  // The dynamically constructed URL for downloading the JSON.
  private downloadURL: string = '';

  private editor: JSONEditor | null = null;

  private playing: boolean = false;

  private hasEdits: boolean = false;

  private currentNameOrHash: string = '';

  private json: any = defaultParticleDemo;

  private playPauseButton: HTMLButtonElement | null = null;

  private particlesPlayer: ParticlesPlayerSk | null = null;

  // stateReflector update function.
  private stateChanged: stateChangedCallback | null = null;

  private configEditor: ParticlesConfigSk | null = null;

  private editorDetails: HTMLDetailsElement | null = null;

  private widthInput: HTMLInputElement | null = null;

  private heightInput: HTMLInputElement | null = null;

  constructor() {
    super(ParticlesSk.template);
  }

  private static template = (ele: ParticlesSk) => html`
  <header>
    <h2>Particles</h2>
    <span>
      <a
        id=githash
        href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'
      >
        ${SKIA_VERSION.slice(0, 7)}
      </a>
      <theme-chooser-sk></theme-chooser-sk>
    </span>
  </header>
  <main>
    <particles-config-sk></particles-config-sk>
    <!-- TODO(jcgregorio) Eventually this should be replaced with Scrap Exchange List Names. -->
    <span @click=${ele.namedDemoLinkClick}>
      <a href="/?nameOrHash=@fireworks">fireworks</a>
      <a href="/?nameOrHash=@spiral">spiral</a>
      <a href="/?nameOrHash=@swirl">swirl</a>
      <a href="/?nameOrHash=@text">text</a>
      <a href="/?nameOrHash=@wave">wave</a>
      <a href="/?nameOrHash=@cube">cube</a>
      <a href="/?nameOrHash=@confetti">confetti</a>
      <a href="/?nameOrHash=@uniforms">uniforms</a>
    </span>

    <button @click=${ele.openUploadDialog}>
      Upload
    </button>
    <div class=playerAndEditor>
      <figure>
        <particles-player-sk width=${ele.state.width} height=${ele.state.height}></particles-player-sk>
        <figcaption>
          <div>
            <button @click=${ele.restartAnimation}>Restart</button>
            <button id=playpause @click=${ele.togglePlayPause}>Pause</button>
            <button @click=${ele.resetView}>
              Reset Pan/Zoom
            </button>
          </div>
          <div>
            Click to pan. Scroll wheel to zoom.
          </div>
          <div class=download>
            <a target=_blank download="particles.json" href=${ele.downloadURL}>
              Download JSON
            </a>
            ${ele.hasEdits ? '(without edits)' : ''}
          </div>
        </figcaption>
      </figure>
      <div>
        <details id=editorDetails
          ?open=${ele.state.showEditor}
          @toggle=${ele.toggleEditor}>
          <summary>Edit</summary>
          <div id=dimensions>
            <label>
              <input
               id=width
               type=number
               .value=${ele.state.width.toFixed(0)}
               @change=${ele.widthChange}}
              /> Width (px)
            </label>
            <label>
              <input
                id=height
                type=number
                .value=${ele.state.height.toFixed(0)}
                @change=${ele.heightChange}
              /> Height (px)
            </label>
          </div>
          <div id=json_editor></div>
        </details>
      </div>
      <button ?hidden=${!ele.hasEdits} @click=${ele.applyEdits}>Apply Edits</button>
    </div>
  </main>
  <footer>
    <error-toast-sk></error-toast-sk>
  </footer>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    const editorContainer = $$<HTMLDivElement>('#json_editor', this)!;
    // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
    // for documentation on this editor.
    const editorOptions: JSONEditorOptions = {
      sortObjectKeys: true,
      onChange: () => {
        this.hasEdits = true;
        this._render();
      },
    };
    this.editor = new JSONEditor(editorContainer, editorOptions);
    this.widthInput = $$<HTMLInputElement>('#width', this);
    this.heightInput = $$<HTMLInputElement>('#height', this);
    this.particlesPlayer = $$<ParticlesPlayerSk>('particles-player-sk', this);
    this.playPauseButton = $$<HTMLButtonElement>('#playpause', this);
    this.configEditor = $$<ParticlesConfigSk>('particles-config-sk', this);
    this.editorDetails = $$<HTMLDetailsElement>('#editorDetails', this);

    this.stateChanged = stateReflector(
      /* getState */() => (this.state as unknown as HintableObject),
      /* setState */(newState) => {
        this.state = (newState as unknown as State);
        this._render();
        this.loadParticlesIfNecessary();
      },
    );

    this.setJSON(defaultParticleDemo);
    this.editor!.expandAll();
    if (this.hasAttribute('paused')) {
      this.pause();
    } else {
      this.play();
    }
  }

  // We can make the links to named demo pages faster by intercepting the link
  // click and just changing this.state, which avoids a page load.
  private namedDemoLinkClick(e: MouseEvent) {
    const url = (e.target as HTMLLinkElement).href;
    if (!url) {
      return;
    }
    const parsedURL = new URL(url);
    const nameOrHash = parsedURL.searchParams.get('nameOrHash');
    if (!nameOrHash) {
      return;
    }
    e.stopPropagation();
    e.preventDefault();
    this.state.nameOrHash = nameOrHash;
    this.stateChanged!();
    this.loadParticlesIfNecessary();
  }

  private widthChange() {
    this.state.width = +this.widthInput!.value;
    this.dimensionsChanged();
  }

  private heightChange() {
    this.state.height = +this.heightInput!.value;
    this.dimensionsChanged();
  }

  private dimensionsChanged() {
    this._render();
    this.stateChanged!();
    this.particlesPlayer!.initialize({
      body: this.json,
      width: this.state.width,
      height: this.state.height,
    });
  }

  private setJSON(json: any) {
    this.json = json;
    this.editor!.update(json);
    this.particlesPlayer!.initialize({
      body: this.json,
      width: this.state.width,
      height: this.state.height,
    });
    if (this.downloadURL) {
      URL.revokeObjectURL(this.downloadURL);
    }

    this.downloadURL = URL.createObjectURL(new Blob([JSON.stringify(this.json, null, '  ')]));
    this.hasEdits = false;
    this._render();
  }

  private async openUploadDialog() {
    try {
      const cfg: ParticlesConfig = {
        body: this.json,
      };
      const newConfig = await this.configEditor!.show(cfg);
      if (!newConfig) {
        return;
      }
      this.setJSON(newConfig.body);
      await this.upload();
      this.stateChanged!();
      this._render();
    } catch (err) {
      await errorMessage(err);
    }
  }

  private async applyEdits() {
    this.setJSON(this.editor!.get());
    await this.upload();
  }

  private togglePlayPause() {
    if (this.playing) {
      this.pause();
    } else {
      this.play();
    }
  }

  private play() {
      this.playPauseButton!.textContent = 'Pause';
      this.particlesPlayer!.play();
      this.playing = true;
  }

  private pause() {
    this.playPauseButton!.textContent = 'Play';
    this.particlesPlayer!.pause();
    this.playing = false;
  }

  private async loadParticlesIfNecessary() {
    try {
      if (this.currentNameOrHash === this.state.nameOrHash) {
        return;
      }
      const resp = await fetch(`/_/j/${this.state.nameOrHash}`, {
        credentials: 'include',
      });
      const json = await jsonOrThrow(resp) as ScrapBody;
      this.setJSON(JSON.parse(json.Body) as any);
      this.play();
      this.currentNameOrHash = this.state.nameOrHash;
    } catch (error) {
      await errorMessage(error);
      // Return to the default view.
      this.state = Object.assign({}, defaultState);
      this.currentNameOrHash = this.state.nameOrHash;
      this.stateChanged!();
    }
  }

  private resetView() {
    this.particlesPlayer!.resetView();
  }

  private restartAnimation() {
    this.particlesPlayer!.restartAnimation();
  }

  private toggleEditor() {
    this.state.showEditor = this.editorDetails!.open;
    this.stateChanged!();
    this._render();
  }

  private async upload() {
    const body: ScrapBody = {
      Body: JSON.stringify(this.json),
      Type: 'particle',
    };
    try {
      // POST the JSON to /_/upload
      const resp = await fetch('/_/upload', {
        credentials: 'include',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
        method: 'POST',
      });
      const json = await jsonOrThrow(resp) as ScrapID;

      this.state.nameOrHash = json.Hash;
      this.stateChanged!();
    } catch (error) {
      await errorMessage(`${error}`);
    }
  }
}

define('particles-sk', ParticlesSk);
