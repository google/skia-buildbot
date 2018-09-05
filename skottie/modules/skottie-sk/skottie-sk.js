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
import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

const JSONEditor = require('jsoneditor/dist/jsoneditor-minimalist.js');
const bodymovin = require('lottie-web/build/player/lottie.min.js');

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;

const displayDialog = (ele) => html`
<skottie-config-sk state=${ele._state}></skottie-config-sk>
`;

const video = (ele) => {
  // ele._hash is filled in when the video has been loaded from the server
  // and blank otherwise.
  if (ele._hash) {
    return html`
<video id=video muted loop on-loadeddata=${(e) => ele._videoLoaded(e)} title=lottie
    src='/_/i/${ele._hash}' width=${ele._state.width} height=${ele._state.height}>
  Your browser does not support the video tag.
</video>

<figcaption>skottie</figcaption>`;
  } else {
    // Have a blank video to make it look like something should be there
    // but don't set the source to avoid a 404 request to an empty video
    // (e.g. when hash is empty string).
    return html`
<video title=loading muted loop
       width=${ele._state.width} height=${ele._state.height}>
</video>

<figcaption>
  <div class=video_loading title="We are processing the skottie video on the backend.">
    <div>Processing</div>
    <spinner-sk active></spinner-sk>
  </div>
  <span>skottie</span>
</figcaption>
`;
  }
}

const livePreview = (ele) => {
  if (ele._hasEdits) {
    return html`
<figure>
  <div id=live title=live-preview
       style='width: ${ele._state.width}px; height: ${ele._state.height}px'></div>
  <figcaption>Preview [lottie-web]</figcaption>
</figure>`;
  } else {
    return '';
  }
}

const displayLoaded= (ele) => html`
<button class=edit-config on-click=${(e) => ele._startEdit()}>${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...</button>
<button on-click=${(e) => ele._rewind(e)}>Rewind</button>
<button id=playpause on-click=${(e) => ele._playpause(e)}>Pause</button>
<button hidden?=${!ele._hasEdits} on-click=${(e) => ele._applyEdits()}>Apply Edits</button>
<div class=download>
  <a target=_blank download=${ele._state.filename} href=${ele._downloadUrl}>
    JSON
  </a>
  ${ele._hasEdits? '(without edits)': ''}
</div>
<section class=figures>
  <figure>
    ${video(ele)}
  </figure>
  <figure>
    <div id=container title=lottie-web
         style='width: ${ele._state.width}px; height: ${ele._state.height}px'></div>
    <figcaption>lottie-web (${bodymovin.version})</figcaption>
  </figure>
  ${livePreview(ele)}
</section>

<section class=editor>
  <div id=json_editor></div>
</section>
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
  <h2>Skottie</h2><span><a href='https://skia.googlesource.com/skia/+/${ele.version}'>${ele.version.slice(0, 7)}</a></span>
</header>
<main>
  ${pick(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
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
    this._lottie = null;
    this._live = null;
    this._video = null;
    this._playing = true;
    this._downloadUrl = null; // The URL to download the lottie JSON from.
    this._editor = null;
    this._editorLoaded = false;
    this._hasEdits = false;
  }

  connectedCallback() {
    this._reflectFromURL();
    this.addEventListener('skottie-selected', this)
    this.addEventListener('cancelled', this)
    window.addEventListener('popstate', this)
    this._render();
  }

  disconnectedCallback() {
    this.removeEventListener('skottie-selected', this)
    this.removeEventListener('cancelled', this)
  }

  static get observedAttributes() {
    return ['version'];
  }

  /** @prop version {string} The version of Skia. */
  get version() { return this.getAttribute('version'); }
  set version(val) { this.setAttribute('version', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
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
    this._render();
    fetch(`/_/j/${this._hash}`, {
      credentials: 'include',
    }).then(jsonOrThrow).then(json => {
      this._state = json;
      this._ui = LOADED_MODE;
      this._render();
    }).catch((msg) => {
      errorMessage(msg);
      window.history.pushState(null, '', '/');
      this._ui = DIALOG_MODE;
      this._render();
    });
  }

  _applyEdits() {
    if (!this._editor || !this._editorLoaded || !this._hasEdits) {
      return;
    }
    this._state.lottie = this._editor.get();
    this._upload();
  }

  _startEdit() {
    this._ui = DIALOG_MODE;
    this._render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._hash = '';
    this._hasEdits = false;
    this._editorLoaded = false;
    this._editor = null;
    this._render();
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
      this._render();
      // Re-sync all players
      this._rewind();
    }).catch(msg => {
      errorMessage(msg);
      window.history.pushState(null, '', '/');
      this._ui = DIALOG_MODE;
      this._render();
    });
    this._ui = LOADED_MODE;
    this._render();
  }

  _playpause(e) {
    if (this._playing) {
      this._lottie.pause();
      this._video.pause();
      this._live && this._live.pause();
      e.target.textContent = 'Play';
    } else {
      this._lottie.play();
      this._video.play();
      this._live && this._live.play();
      e.target.textContent = 'Pause';
    }
    this._playing = !this._playing;
  }

  _rewind(e) {
    if (!this._playing) {
      this._live && this._live.goToAndStop(0);
      this._lottie.goToAndStop(0);
      this._video.currentTime = 0;
    } else {
      this._live && this._live.goToAndPlay(0);
      this._lottie.goToAndPlay(0);
      this._video.currentTime = 0;
      this._video.play();
    }
  }

  _videoLoaded(e) {
    e.target.play();
    this._playing = true;
    this._rewind();
  }

  _render() {
    if (this._downloadUrl)  {
      URL.revokeObjectURL(this._downloadUrl);
    }
    this._downloadUrl = URL.createObjectURL(new Blob([JSON.stringify(this._state.lottie, null, '  ')]));
    render(template(this), this);
    if (this._ui == LOADED_MODE) {
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
        this._video = $$('#video', this);
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

      // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
      // for documentation on this editor.
      let editorOptions = {
        sortObjectKeys: true,
        // There are sometimes a few onChange events that happen
        // during the initial .set(), so we have a safety variable
        // _editorLoaded to prevent a bunch of recursion and
        onChange: () => {
          if (!this._editorLoaded) {
            return;
          }
          this._hasEdits = true;
          this._render();
        }
      };

      if (!this._editor) {
        this._editorLoaded = false;
        $$('#json_editor').innerHTML = '';
        this._editor = new JSONEditor($$('#json_editor'), editorOptions);
      }
      if (!this._hasEdits) {
        // Only set the JSON when it is loaded, either because it's
        // the first time we got it from the server or because the user
        // hit applyEdits.
        this._editor.set(this._state.lottie);
      }
      // We are now pretty confident that the onChange events will only be
      // when the user modifies the JSON.
      this._editorLoaded = true;
    }
  }

  handleEvent(e) {
    if (e.type == 'skottie-selected') {
      this._state = e.detail;
      this._upload();
    } else if (e.type == 'cancelled') {
      this._ui = LOADED_MODE;
      this._editor = null;
      this._render();
    } else if (e.type == 'popstate') {
      this._reflectFromURL();
    }
  }

});
