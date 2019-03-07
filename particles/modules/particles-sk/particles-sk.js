/**
 * @module particles-sk
 * @description <h2><code>particles-sk</code></h2>
 *
 * <p>
 *   The main application element for particles.
 * </p>
 *
 */
import '../particles-player-sk'
import 'elements-sk/checkbox-sk'
import 'elements-sk/error-toast-sk'
import 'elements-sk/styles/buttons'
import { $$ } from 'common-sk/modules/dom'
import { SKIA_VERSION } from '../../build/version.js'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { stateReflector } from 'common-sk/modules/stateReflector'

// FIXME(kjlubick): remove this placeholder
import { spiral } from './test_data.js'

const JSONEditor = require('jsoneditor/dist/jsoneditor-minimalist.js');

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;


// SCRUBBER_RANGE is the input range for the scrubbing control.
const SCRUBBER_RANGE = 1000;

const displayDialog = (ele) => html`
<div> TODO config / dialog </div>
`;

const particlesPlayer = (ele) => html`
<particles-player-sk width=${ele._width} height=${ele._height}>
</particles-player-sk>

<figcaption>
  particles-wasm
</figcaption>`;

const jsonEditor = (ele) => {
  if (!ele._showEditor) {
    return '';
  }
  return html`
<section class=editor>
  <div id=json_editor></div>
</section>`;
}

const displayLoaded = (ele) => html`
<button class=edit-config @click=${ ele._startEdit}>
  ${ele._state.filename} ${ele._width}x${ele._height} ...
</button>
<div class=controls>
  <button @click=${ele._reset}>Reset</button>
  <button id=playpause @click=${ele._playpause}>Pause</button>
  <button ?hidden=${!ele._hasEdits} @click=${ele._applyEdits}>Apply Edits</button>
  <div class=download>
    <a target=_blank download=${ele._state.filename} href=${ele._downloadUrl}>
      JSON
    </a>
    ${ele._hasEdits? '(without edits)': ''}
  </div>
  <checkbox-sk label="Show editor"
               ?checked=${ele._showEditor}
               @click=${ele._toggleEditor}>
  </checkbox-sk>
</div>

<section class=figures>
  <figure>
    ${particlesPlayer(ele)}
  </figure>
</section>

${jsonEditor(ele)}
`;

const displayLoading = (ele) => html`
<div class=loading>
  <spinner-sk active></spinner-sk><span>Loading...</span>
</div>
`;

// pick the right part of the UI to display based on ele._ui.
const pick = (ele) => {
  switch (ele._ui) {
    case DIALOG_MODE:
      return displayDialog(ele);
    case LOADING_MODE:
      return displayLoading(ele);
    case LOADED_MODE:
      return displayLoaded(ele);
  }
};

const template = (ele) => html`
<header>
  <h2>Particles</h2><span><a href='https://www.npmjs.com/package/canvaskit-wasm/v/${SKIA_VERSION}'>${SKIA_VERSION.slice(0, 7)}</a></span>
</header>
<main>
  ${pick(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

window.customElements.define('particles-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      json: null,
    };
    // One of 'dialog', 'loading', or 'loaded'
    this._ui = DIALOG_MODE;
    this._hash = '';
    this._playing = true;
    this._downloadUrl = null; // The URL to download the particles JSON from.
    this._editor = null;
    this._editorLoaded = false;
    this._hasEdits = false;
    this._showEditor = false;

    this._width = 0;
    this._height = 0;

    this._stateChanged = stateReflector(
      /*getState*/() => {
        return {
          // provide empty values
          'e' : this._showEditor,
          'w' : this._width,
          'h' : this._height,
        }
    }, /*setState*/(newState) => {
      this._showEditor = newState.e;
      this._width = newState.w;
      this._height = newState.h;
      if (!this._width) {
        this._width = Math.min(800, window.outerWidth * .9);
      }
      if (!this._height) {
        this._height = Math.min(800, window.outerHeight * .9);
      }
      this._reflectFromURL();
      this.render();
    });

    this._playerLoaded = false;

    this._player = {
      surface: null,
      canvas: null,
      particles: null,
    }

    // The wasm animation computes how long it has been since it started and
    // use arithmetic to figure out where to seek (i.e. which frame to draw).
    this._firstFrameTime = null;
     // used for remembering where we were in the animation when paused.
    this._wasmTimePassed = 0;
  }

  connectedCallback() {
    this.addEventListener('particles-json-selected', this)
    this.addEventListener('cancelled', this)
    window.addEventListener('popstate', this)
    this.render();
  }

  disconnectedCallback() {
    this.removeEventListener('particles-json-selected', this)
    this.removeEventListener('cancelled', this)
  }

  attributeChangedCallback(name, oldValue, newValue) {
    this.render();
  }

  _applyEdits() {
    if (!this._editor || !this._editorLoaded || !this._hasEdits) {
      return;
    }
    this._state.json = this._editor.get();
    this._upload();
  }

  handleEvent(e) {
    if (e.type === 'particles-json-selected') {
      this._state = e.detail.state;
      this._width = e.detail.width;
      this._height = e.detail.height;
      this._stateChanged();
      if (e.detail.fileChanged) {
        this._upload();
      } else {
        this._ui = LOADED_MODE;
        this.render();
        this._initializePlayer();
        // Re-sync all players
        this._reset();
      }
    } else if (e.type === 'cancelled') {
      this._ui = LOADED_MODE;
      this.render();
      this._initializePlayer();
    } else if (e.type === 'popstate') {
      this._reflectFromURL();
    }
  }

  _initializePlayer() {
    this._particlesPlayer.initialize({
      width: this._width,
      height: this._height,
      json: this._state.json,
    });
    this._playerLoaded = true;
  }

  _playpause() {
    if (this._playing) {
      $$('#playpause').textContent = 'Play';
      this._particlesPlayer.pause();
    } else {
      $$('#playpause').textContent = 'Pause';
      this._particlesPlayer.play();
    }
    this._playing = !this._playing;
  }

  _reflectFromURL() {
    // Check URL.
    let match = window.location.pathname.match(/\/([a-zA-Z0-9]+)/);
    if (!match) {
      // Make this the hash of the particles file you want to play on startup.
      this._hash = '1112d01d28a776d777cebcd0632da15b'; // spiral.json
    } else {
      this._hash = match[1];
    }
    this._ui = LOADING_MODE;
    this.render();
    // TODO(kjlubick) Actually make a fetch request
    this._state.json = spiral;
    this._state.filename = 'spiral.json';
    this._ui = LOADED_MODE;
    this.render();
    this._initializePlayer();
    // Force start playing
    this._playing = false;
    this._playpause();
  }

  render() {
    if (this._downloadUrl)  {
      URL.revokeObjectURL(this._downloadUrl);
    }
    this._downloadUrl = URL.createObjectURL(new Blob([JSON.stringify(this._state.json, null, '  ')]));
    render(template(this), this, {eventContext: this});

    this._particlesPlayer = $$('particles-player-sk', this);

    if (this._ui === LOADED_MODE) {
      try {
        this._renderJSONEditor();
      } catch(e) {
        console.warn('caught error while rendering third party code', e);
      }

    }
  }

  _renderJSONEditor() {
    if (!this._showEditor) {
      this._editorLoaded = false;
      this._editor = null;
      this._hasEdits = false;
      return;
    }
    let editorContainer = $$('#json_editor');
    // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
    // for documentation on this editor.
    let editorOptions = {
      sortObjectKeys: true,
      // There are sometimes a few onChange events that happen
      // during the initial .set(), so we have a safety variable
      // _editorLoaded to prevent a bunch of recursion
      onChange: () => {
        if (!this._editorLoaded) {
          return;
        }
        this._hasEdits = true;
        this.render();
      }
    };

    if (!this._editor) {
      this._editorLoaded = false;
      editorContainer.innerHTML = '';
      this._editor = new JSONEditor(editorContainer, editorOptions);
    }
    if (!this._hasEdits) {
      this._editorLoaded = false;
      // Only set the JSON when it is loaded, either because it's
      // the first time we got it from the server or because the user
      // hit applyEdits.
      this._editor.set(this._state.json);
    }
    // We are now pretty confident that the onChange events will only be
    // when the user modifies the JSON.
    this._editorLoaded = true;
  }

  _reset() {
    this._particlesPlayer && this._particlesPlayer.reset();
  }

  _startEdit() {
    this._ui = DIALOG_MODE;
    this.render();
  }

  _toggleEditor(e) {
    // avoid double toggles
    e.preventDefault();
    this._showEditor = !this._showEditor;
    this._stateChanged();
    this.render();
  }

  _upload() {
    // POST the JSON to /_/upload
    this._hash = '';
    this._hasEdits = false;
    this._editorLoaded = false;
    this._editor = null;
    // Clean up the old animation and other wasm objects
    this.render();
    console.log('should upload JSON')

    this._ui = LOADED_MODE;
    // Start drawing right away, no need to wait for
    // the JSON to make a round-trip to the server.
    this.render();
    this._initializePlayer();
  }

});
