/**
 * @module skottie-sk
 * @description <h2><code>skottie-sk</code></h2>
 *
 * <p>
 *   The main application element for skottie.
 * </p>
 *
 */
import '../skottie-config-sk'
import '../skottie-player-sk'
import 'elements-sk/checkbox-sk'
import 'elements-sk/collapse-sk'
import 'elements-sk/error-toast-sk'
import { $$ } from 'common-sk/modules/dom'
import { SKIA_VERSION } from '../../build/version.js'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { setupListeners, onUserEdit, reannotate} from '../lottie-annotations'
import { stateReflector } from 'common-sk/modules/stateReflector'

const JSONEditor = require('jsoneditor/dist/jsoneditor-minimalist.js');
const bodymovin = require('lottie-web/build/player/lottie.min.js');

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;

const displayDialog = (ele) => html`
<skottie-config-sk .state=${ele._state}></skottie-config-sk>
`;

const skottiePlayer = (ele) => html`
<skottie-player-sk paused width=${ele._state.width} height=${ele._state.height}>
</skottie-player-sk>

<figcaption>
  skottie-wasm
</figcaption>`;

const lottiePlayer = (ele) => {
  if (!ele._showLottie) {
    return '';
  }
  return html`
<figure>
  <div id=container title=lottie-web
       style='width: ${ele._state.width}px; height: ${ele._state.height}px'></div>
  <figcaption>lottie-web (${bodymovin.version})</figcaption>
</figure>`;
}

// TODO(kjlubick): Make the live preview use skottie
const livePreview = (ele) => {
  if (!ele._hasEdits || !ele._showLottie) {
    return '';
  }
  if (ele._hasEdits) {
    return html`
<figure>
  <div id=live title=live-preview
       style='width: ${ele._state.width}px; height: ${ele._state.height}px'></div>
  <figcaption>Preview [lottie-web]</figcaption>
</figure>`;
  }
}

const iframeDirections = (ele) => {
  return `<iframe width="${ele._state.width}" height="${ele._state.height}" src="${window.location.origin}/e/${ele._hash}" scrolling=no>`;
}

const inlineDirections = (ele) => {
  return `<skottie-inline-sk width="${ele._state.width}" height="${ele._state.height}" src="${window.location.origin}/_/j/${ele._hash}"></skottie-inline-sk>`;
}

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
  ${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...
</button>
<div class=controls>
  <button @click=${ele._rewind}>Rewind</button>
  <button id=playpause @click=${ele._playpause}>Pause</button>
  <button ?hidden=${!ele._hasEdits} @click=${ele._applyEdits}>Apply Edits</button>
  <div class=download>
    <a target=_blank download=${ele._state.filename} href=${ele._downloadUrl}>
      JSON
    </a>
    ${ele._hasEdits? '(without edits)': ''}
  </div>
  <checkbox-sk label="Show lottie-web"
               ?checked=${ele._showLottie}
               @click=${ele._toggleLottie}>
  </checkbox-sk>
  <checkbox-sk label="Show editor"
               ?checked=${ele._showEditor}
               @click=${ele._toggleEditor}>
  </checkbox-sk>
  <button @click=${ele._toggleEmbed}>Embed</button>
</div>
<collapse-sk id=embed closed>
  <p>
    <label>
      Embed using an iframe: <input size=120 value=${iframeDirections(ele)} scrolling=no>
    </label>
  </p>
  <p>
    <label>
      Embed on skia.org: <input size=140 value=${inlineDirections(ele)} scrolling=no>
    </label>
  </p>
</collapse-sk>

<section class=figures>
  <figure>
    ${skottiePlayer(ele)}
  </figure>
  ${lottiePlayer(ele)}
  ${livePreview(ele)}
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

const redir = (ele) => {
  if (window.location.hostname !== 'skottie-internal.skia.org') {
    return html`<div>Googlers should use <a href="https://skottie-internal.skia.org">skottie-internal.skia.org</a>.</div>`;
  } else {
    return html``;
  }
};

const template = (ele) => html`
<header>
  <h2>Skottie</h2><span><a href='https://www.npmjs.com/package/canvaskit-wasm/v/${SKIA_VERSION}'>${SKIA_VERSION.slice(0, 7)}</a></span>
</header>
<main>
  ${pick(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
  ${redir(ele)}
</footer>
`;

window.customElements.define('skottie-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: null,
      width: 256,
      height: 256,
      fps: 30,
    };
    // One of 'dialog', 'loading', or 'loaded'
    this._ui = DIALOG_MODE;
    this._hash = '';
    this._skottiePlayer = null;
    this._lottie = null;
    this._live = null;
    this._playing = true;
    this._downloadUrl = null; // The URL to download the lottie JSON from.
    this._editor = null;
    this._editorLoaded = false;
    this._hasEdits = false;
    this._showLottie = false;
    this._showEditor = false;

    this._stateChanged = stateReflector(
      /*getState*/() => {
        return {
          // provide empty values
          'l' : this._showLottie,
          'e' : this._showEditor,
        }
    }, /*setState*/(newState) => {
      this._showLottie = newState.l;
      this._showEditor = newState.e;
      this.render();
    });

    this._duration = 0; // _duration = 0 is a sentinel value for "player not loaded yet"

    // The wasm animation computes how long it has been since it started and
    // use arithmetic to figure out where to seek (i.e. which frame to draw).
    this._firstFrameTime = null;
     // used for remembering where we were in the animation when paused.
    this._wasmTimePassed = 0;
  }

  connectedCallback() {
    this._reflectFromURL();
    this.addEventListener('skottie-selected', this)
    this.addEventListener('cancelled', this)
    window.addEventListener('popstate', this)
    this.render();

    // Start a continous animation loop.
    const drawFrame = () => {
      window.requestAnimationFrame(drawFrame);

      // Elsewhere, the _firstFrameTime is set to null to restart
      // the animation. If null, we assume the user hit re-wind
      // and restart both the Skottie animation and the lottie-web one.
      // This avoids the (small) boot-up lag while we wait for the
      // skottie animation to be parsed and loaded.
      if (!this._firstFrameTime && this._playing) {
        this._firstFrameTime = Date.now();
      }
      if (this._playing && this._duration > 0) {
        let progress = (Date.now() - this._firstFrameTime) % this._duration;

        // If we want to have synchronized playing, it's best to force
        // all players to draw the same frame rather than letting them play
        // on their own timeline.
        this._skottiePlayer && this._skottiePlayer.seek(progress / this._duration);

        this._lottie && this._lottie.goToAndStop(progress);
        this._live && this._live.goToAndStop(progress);
      }
    }

    window.requestAnimationFrame(drawFrame);
  }

  disconnectedCallback() {
    this.removeEventListener('skottie-selected', this)
    this.removeEventListener('cancelled', this)
  }

  attributeChangedCallback(name, oldValue, newValue) {
    this.render();
  }

  _applyEdits() {
    if (!this._editor || !this._editorLoaded || !this._hasEdits) {
      return;
    }
    this._state.lottie = this._editor.get();
    this._upload();
  }

  handleEvent(e) {
    if (e.type === 'skottie-selected') {
      this._state = e.detail;
      this._upload();
    } else if (e.type === 'cancelled') {
      this._ui = LOADED_MODE;
      this.render();
      this._initializePlayer();
    } else if (e.type === 'popstate') {
      this._reflectFromURL();
    }
  }

  _initializePlayer() {
    this._skottiePlayer.initialize({
      width:  this._state.width,
      height: this._state.height,
      lottie: this._state.lottie,
    }).then(() => {
      this._duration = this._skottiePlayer.duration();
    });
  }

  _playpause() {
    if (this._playing) {
      this._wasmTimePassed = Date.now() - this._firstFrameTime;
      this._lottie && this._lottie.pause();
      this._live && this._live.pause();
      $$('#playpause').textContent = 'Play';
    } else {
      this._lottie && this._lottie.play();
      this._live && this._live.play();
      this._firstFrameTime = Date.now() - (this._wasmTimePassed || 0);
      $$('#playpause').textContent = 'Pause';
    }
    this._playing = !this._playing;
  }

  _reflectFromURL() {
    // Check URL.
    let match = window.location.pathname.match(/\/([a-zA-Z0-9]+)/);
    if (!match) {
      // Make this the hash of the lottie file you want to play on startup.
      this._hash = '1112d01d28a776d777cebcd0632da15b'; // gear.json
    } else {
      this._hash = match[1];
    }
    this._ui = LOADING_MODE;
    this.render();
    // Run this on the next micro-task to allow mocks to be set up if needed.
    setTimeout(() => {
      fetch(`/_/j/${this._hash}`, {
        credentials: 'include',
      }).then(jsonOrThrow).then(json => {
        this._state = json;
        this._ui = LOADED_MODE;
        this.render();
        this._initializePlayer();
      }).catch((msg) => {
        errorMessage(msg);
        window.history.pushState(null, '', '/');
        this._ui = DIALOG_MODE;
        this.render();
      });
    });
  }

  render() {
    if (this._downloadUrl)  {
      URL.revokeObjectURL(this._downloadUrl);
    }
    this._downloadUrl = URL.createObjectURL(new Blob([JSON.stringify(this._state.lottie, null, '  ')]));
    render(template(this), this, {eventContext: this});

    if (this._ui === LOADED_MODE) {
      this._renderLottieWeb();
      this._renderJSONEditor();
    }
    this._skottiePlayer = $$('skottie-player-sk', this);
  }

  _renderJSONEditor() {
    if (!this._showEditor) {
      this._editorLoaded = false;
      this._editor = null;
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
        onUserEdit(editorContainer, this._editor.get());
        this.render();
      }
    };

    if (!this._editor) {
      this._editorLoaded = false;
      editorContainer.innerHTML = '';
      this._editor = new JSONEditor(editorContainer, editorOptions);
      setupListeners(editorContainer);
    }
    if (!this._hasEdits) {
      this._editorLoaded = false;
      // Only set the JSON when it is loaded, either because it's
      // the first time we got it from the server or because the user
      // hit applyEdits.
      this._editor.set(this._state.lottie);
    }
    reannotate(editorContainer, this._state.lottie);
    // We are now pretty confident that the onChange events will only be
    // when the user modifies the JSON.
    this._editorLoaded = true;
  }

  _renderLottieWeb() {
    if (!this._showLottie) {
      return;
    }
    // Don't re-start the animation while the user edits.
    if (!this._hasEdits) {
      $$('#container').innerHTML = '';
      this._lottie = bodymovin.loadAnimation({
        container: $$('#container'),
        renderer: 'svg',
        loop: true,
        autoplay: this._playing,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this._state.lottie)),
        rendererSettings: {
          preserveAspectRatio:'xMidYMid meet'
        },
      });
      this._live = null;
    } else {
      // we have edits, update the live preview version.
      // It will re-start from the very beginning, but the user can
      // hit "rewind" to re-sync them.
      $$('#live').innerHTML = '';
      this._live = bodymovin.loadAnimation({
        container: $$('#live'),
        renderer: 'svg',
        loop: true,
        autoplay: this._playing,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this._editor.get())),
        rendererSettings: {
          preserveAspectRatio:'xMidYMid meet'
        },
      });
    }
  }

  _rewind(e) {
    // Handle rewinding when paused.
    this._wasmTimePassed = 0;
    if (!this._playing) {
      this._live && this._live.goToAndStop(0);
      this._lottie && this._lottie.goToAndStop(0);
      this._firstFrameTime = null;
      this._skottiePlayer.seek(0);
    } else {
      this._live && this._live.goToAndPlay(0);
      this._lottie && this._lottie.goToAndPlay(0);
      this._firstFrameTime = null;
    }
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

  _toggleEmbed() {
    let collapse = $$('#embed', this);
    collapse.closed = !collapse.closed;
  }

  _toggleLottie(e) {
    // avoid double toggles
    e.preventDefault();
    this._showLottie = !this._showLottie;
    this._stateChanged();
    this.render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._hash = '';
    this._hasEdits = false;
    this._editorLoaded = false;
    this._editor = null;
    // Clean up the old animation and other wasm objects
    this.render();
    fetch('/_/upload', {
      credentials: 'include',
      body: JSON.stringify(this._state),
      headers: {
        'Content-Type': 'application/json'
      },
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with the hash.
      this._ui = LOADED_MODE;
      this._hash = json.hash;
      window.history.pushState(null, '', '/' + this._hash);
      this.render();
    }).catch(msg => {
      errorMessage(msg);
      window.history.pushState(null, '', '/');
      this._ui = DIALOG_MODE;
      this.render();
    });
    this._ui = LOADED_MODE;
    // Start drawing right away, no need to wait for
    // the JSON to make a round-trip to the server.
    this.render();
    this._initializePlayer();

    // Re-sync all players
    this._rewind();
  }

});
