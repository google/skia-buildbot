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
import JSONEditor from 'jsoneditor';
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
    '  bool explode = p.scale == 1;',
    '',
    '  p.lifetime = explode ? (2 + rand(p.seed) * 0.5) : 0.5;',
    '  float a = radians(rand(p.seed) * 360);',
    '  float s = explode ? mix(90, 100, rand(p.seed)) : mix(5, 10, rand(p.seed));',
    '  p.vel.x = cos(a) * s;',
    '  p.vel.y = sin(a) * s;',
    '}',
    '',
    'void update(inout Particle p) {',
    '  p.color.a = 1 - p.age;',
    '  if (p.scale == 1) {',
    '    p.vel.y += dt * 50;',
    '  }',
    '}',
    '',
  ],
  Drawable: {
    Radius: 3,
    Type: 'SkCircleDrawable',
  },
  EffectCode: [
    'void effectSpawn(inout Effect effect) {',
    '  // Phase one: Launch',
    '  effect.lifetime = 4;',
    '  effect.rate = 120;',
    '  float a = radians(mix(-20, 20, rand(effect.seed)) - 90);',
    '  float s = mix(200, 220, rand(effect.seed));',
    '  effect.vel.x = cos(a) * s;',
    '  effect.vel.y = sin(a) * s;',
    '  effect.color.rgb = float3(rand(effect.seed), rand(effect.seed), rand(effect.seed));',
    '  effect.pos.x = 0;',
    '  effect.pos.y = 0;',
    '  effect.scale = 0.25;  // Also used as particle behavior flag',
    '}',
    '',
    'void effectUpdate(inout Effect effect) {',
    '  if (effect.age > 0.5 && effect.rate > 0) {',
    '    // Phase two: Explode',
    '    effect.rate = 0;',
    '    effect.burst = 50;',
    '    effect.scale = 1;',
    '  } else {',
    '    effect.vel.y += dt * 90;',
    '  }',
    '}',
    '',
  ],
  MaxCount: 300,
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

  private configEditor: ParticlesConfigSk |null = null;

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
      ${ele.state.width}x${ele.state.height}
    </button>
    <div class=controls>
      <checkbox-sk label="Show editor"
                  ?checked=${ele.state.showEditor}
                  @input=${ele.toggleEditor}>
      </checkbox-sk>
    </div>
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
      <div id=json_editor ?hidden=${!ele.state.showEditor}></div>
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
    const editorOptions = {
      sortObjectKeys: true,
      onChange: () => {
        this.hasEdits = true;
        this._render();
      },
    };

    this.editor = new JSONEditor(editorContainer, editorOptions);
    this.particlesPlayer = $$<ParticlesPlayerSk>('particles-player-sk', this);
    this.playPauseButton = $$<HTMLButtonElement>('#playpause', this);
    this.configEditor = $$<ParticlesConfigSk>('particles-config-sk', this);

    this.setJSON(defaultParticleDemo);
    this.forceStartPlaying();
  }

  private forceStartPlaying() {
    this.playing = false;
    this.playpauseClick();
  }

  private setJSON(json: any) {
    this.json = json;
    this.editor!.set(json);
    this.particlesPlayer!.initialize({
      width: this.state.width,
      height: this.state.height,
      body: this.json,
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
        width: this.state.width,
        height: this.state.height,
      };
      const newConfig = await this.configEditor!.show(cfg);
      this.state.width = newConfig.width;
      this.state.height = newConfig.height;
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
      this.setJSON(json);
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
    this.state.showEditor = !this.state.showEditor;
    this.stateChanged!();
    this._render();
  }

  private async upload() {
    try {
      // POST the JSON to /_/upload
      const resp = await fetch('/_/upload', {
        credentials: 'include',
        body: JSON.stringify({
          Body: this.json,
        }),
        headers: {
          'Content-Type': 'application/json',
        },
        method: 'POST',
      });
      const json = await jsonOrThrow(resp);

      this.state.nameOrHash = json.hash;
      this.stateChanged!();
    } catch (error) {
      errorMessage(error);
    }
  }
}

define('particles-sk', ParticlesSk);
