/**
 * @module particles-sk
 * @description <h2><code>particles-sk</code></h2>
 *
 * <p>
 *   The main application element for particles.
 * </p>
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
import { SKIA_VERSION } from '../../build/version';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParticlesConfig, ParticlesConfigSk } from '../particles-config-sk/particles-config-sk';
import { ParticlesPlayerSk } from '../particles-player-sk/particles-player-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

const defaultParticleDemo = {
  Bindings: [],
  Code: [
    'void spawn(inout Particle p) {',
    '  p.lifetime = 2 + rand(p.seed);',
    '  p.vel = p.dir * mix(50, 60, rand(p.seed));',
    '}',
    '',
    'void update(inout Particle p) {',
    '  p.scale = 0.5 + 1.5 * p.age;',
    '  float3 a0 = float3(0.098, 0.141, 0.784);',
    '  float3 a1 = float3(0.525, 0.886, 0.980);',
    '  float3 b0 = float3(0.376, 0.121, 0.705);',
    '  float3 b1 = float3(0.933, 0.227, 0.953);',
    '  p.color.rgb = mix(mix(a0, a1, p.age), mix(b0, b1, p.age), rand(p.seed));',
    '}',
    '',
  ],
  Drawable: {
    Radius: 2,
    Type: 'SkCircleDrawable',
  },
  EffectCode: [
    'void effectSpawn(inout Effect effect) {',
    '  effect.lifetime = 4;',
    '  effect.rate = 120;',
    '  effect.spin = 6;',
    '}',
    '',
  ],
  MaxCount: 800,
};

const DEFAULT_SIZE = 800;

interface State {
  showEditor: boolean;
  width: number;
  height: number;
  nameOrHash: string;
}

type stateChangedCallback = ()=> void;

export class ParticlesSk extends ElementSk {
  private state: State = {
    showEditor: true,
    width: DEFAULT_SIZE,
    height: DEFAULT_SIZE,
    nameOrHash: '',
  }

  private downloadURL: string = '';

  private editor: JSONEditor | null = null;

  private playing: boolean = false;

  private hasEdits: boolean = false;

  private currentNameOrHash: string = '';

  private json: any = '';

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

    this.stateChanged = stateReflector(
      /* getState */() => (this.state as unknown as HintableObject),
      /* setState */(newState) => {
        this.state = (newState as unknown as State);
        this._render();
        this.loadParticlesIfNecessary();
      },
    );
  }

  private static template = (ele: ParticlesSk) => html`
  <header>
    <h2>Particles</h2>
    <span>
      <a id=githash
      href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'>
        ${SKIA_VERSION.slice(0, 7)}
      </a>
      <theme-chooser-sk dark></theme-chooser-sk>
    </span>
  </header>
  <main>
    <particles-config-sk></particles-config-sk>
    <a href="/d069873000ab1091296d4c0e561cc622">fireworks</a>
    <a href="/c68434463e7620b60b0bf05f82dc9679">spiral</a>
    <a href="/632d713dacfa01d8905ffee98bc46acc">swirl</a>
    <a href="/9c18c154a286e7c5d64192c9d6661ce0">text</a>
    <a href="/a42f717ffa5f84326e59af238612d1b9">wave</a>

    <button class=edit-config @click=${ele.startEdit}>
      Upload
    </button>
    <div class=playerAndEditor>
      <figure>
        <particles-player-sk width=${ele.state.width} height=${ele.state.height}>  </particles-player-sk>
        <figcaption>
          <div>
            <button @click=${ele.restartAnimation}>Restart</button>
            <button id=playpause @click=${ele.playpauseClick}>Pause</button>
            <button @click=${ele.resetView}>
              Reset Pan/Zoom
            </button>
          </div>
          <div>
           Click to pan, scroll wheel to zoom.
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
               @input=${ele.widthChange}}
              /> Width (px)
            </label>
            <label>
              <input
                id=height
                type=number
                .value=${ele.state.height.toFixed(0)}
                @input=${ele.heightChange}
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

    this.widthInput = $$<HTMLInputElement>('#width', this);
    this.heightInput = $$<HTMLInputElement>('#height', this);
    this.editor = new JSONEditor(editorContainer, editorOptions);
    this.particlesPlayer = $$<ParticlesPlayerSk>('particles-player-sk', this);
    this.playPauseButton = $$<HTMLButtonElement>('#playpause', this);
    this.configEditor = $$<ParticlesConfigSk>('particles-config-sk', this);
    this.editorDetails = $$<HTMLDetailsElement>('#editorDetails', this);

    this.setJSON(defaultParticleDemo);
    this.editor!.expandAll();
    this.forceStartPlaying();
  }

  private widthChange() {
    this.state.width = +this.widthInput!.value;
    this._render();
  }

  private heightChange() {
    this.state.height = +this.heightInput!.value;
    this._render();
  }

  private forceStartPlaying() {
    this.playing = false;
    this.playpauseClick();
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

  private async startEdit() {
    try {
      const cfg: ParticlesConfig = {
        body: this.json,
      };
      const newConfig = await this.configEditor!.show(cfg);
      this.setJSON(newConfig.body);
      this.stateChanged!();
      this._render();
    } catch (err) {
      errorMessage(err);
    }
  }

  private applyEdits() {
    this.setJSON(this.editor!.get());
    this.upload();
  }

  private playpauseClick() {
    if (this.playing) {
      this.playPauseButton!.textContent = 'Play';
      this.particlesPlayer!.pause();
    } else {
      this.playPauseButton!.textContent = 'Pause';
      this.particlesPlayer!.play();
    }
    this.playing = !this.playing;
  }

  private async loadParticlesIfNecessary() {
    try {
      if (this.currentNameOrHash === this.state.nameOrHash) {
        return;
      }
      const resp = await fetch(`/_/j/${this.state.nameOrHash}`, {
        credentials: 'include',
      });
      const json = await jsonOrThrow(resp);
      this.setJSON(JSON.parse(json.Body));
      this.forceStartPlaying();
      this.currentNameOrHash = this.state.nameOrHash;
    } catch (error) {
      errorMessage(error);
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
    try {
      // POST the JSON to /_/upload
      const resp = await fetch('/_/upload', {
        credentials: 'include',
        body: JSON.stringify({
          Body: JSON.stringify(this.json),
        }),
        headers: {
          'Content-Type': 'application/json',
        },
        method: 'POST',
      });
      const json = await jsonOrThrow(resp);

      this.state.nameOrHash = json.Hash;
      this.stateChanged!();
    } catch (error) {
      errorMessage(`${error}`);
    }
  }
}

define('particles-sk', ParticlesSk);
