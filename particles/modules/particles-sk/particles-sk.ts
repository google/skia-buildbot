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
import { SKIA_VERSION } from '../../build/version.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParticlesConfig, ParticlesConfigSk } from '../particles-config-sk/particles-config-sk';
import { ParticlesPlayerSk } from '../particles-player-sk/particles-player-sk';


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

  private json: string = '';

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
      <a href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'>
        ${SKIA_VERSION.slice(0, 7)}
      </a>
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
      <button @click=${ele.restartAnimation}>Restart</button>
      <button id=playpause @click=${ele.playpauseClick}>Pause</button>
      <button ?hidden=${!ele.hasEdits} @click=${ele.applyEdits}>Apply Edits</button>
      <div class=download>
        <a target=_blank download="particles.json" href=${ele.downloadURL}>
          JSON
        </a>
        ${ele.hasEdits ? '(without edits)' : ''}
      </div>
      <checkbox-sk label="Show editor"
                  ?checked=${ele.state.showEditor}
                  @click=${ele.toggleEditor}>
      </checkbox-sk>
      <button @click=${ele.resetView}>
        Reset Pan/Zoom
      </button>
    </div>
    <div class=playerAndEditor>
      <figure>
        <particles-player-sk width=${ele.state.width} height=${ele.state.height}>  </particles-player-sk>
        <figcaption>
          Click to pan, scroll wheel to zoom.
        </figcaption>
      </figure>
      <div id=json_editor></div>
    </div>
  </main>
  <footer>
    <error-toast-sk></error-toast-sk>
  </footer>
  `;


  connectedCallback(): void {
    this._render();
    const editorContainer = $$<HTMLDivElement>('#json_editor')!;
    // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
    // for documentation on this editor.
    const editorOptions = {
      sortObjectKeys: true,
      onChange: () => {
        this.hasEdits = true;
      },
    };

    this.editor = new JSONEditor(editorContainer, editorOptions);
    this.particlesPlayer = $$<ParticlesPlayerSk>('particles-player-sk', this);
    this.playPauseButton = $$<HTMLButtonElement>('#playpause', this);
    this.configEditor = $$<ParticlesConfigSk>('particles-config-sk', this);
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
      this.json = newConfig.body;
      this.stateChanged!();
      this._render();
    } catch (_) {
      // Cancel was pressed.
    }
  }

  private applyEdits() {
    this.json = this.editor!.get();
    this.initializePlayer();
    this.upload();
  }

  private initializePlayer() {
    this.particlesPlayer!.initialize({
      width: this.state.width,
      height: this.state.height,
      body: this.json,
    });
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
      this.json = json.Body;
      this.editor!.set(json);

      if (this.downloadURL) {
        URL.revokeObjectURL(this.downloadURL);
      }

      this.downloadURL = URL.createObjectURL(new Blob([JSON.stringify(this.json, null, '  ')]));
      this._render();

      this.initializePlayer();
      // Force start playing
      this.playing = false;
      this.playpauseClick();
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
