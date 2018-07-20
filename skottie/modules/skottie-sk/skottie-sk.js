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

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;

const displayDialog = (ele) => html`
<skottie-config-sk state=${ele._state}></skottie-config-sk>
`;

const displayLoaded= (ele) => html`
<button class=edit-config on-click=${(e) => ele._startEdit()}>${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...</button>
<button on-click=${(e) => ele._rewind(e)}>Rewind</button>
<button id=playpause on-click=${(e) => ele._playpause(e)}>Pause</button>
<div class=download><a target=_blank download href="https://storage.googleapis.com/skottie-renderer/${ele._hash}/lottie.json">JSON</a></div>
<section class=figures>
  <figure>
    <video id=video muted on-loadeddata=${(e) => ele._videoLoaded(e)} title=lottie loop src='/_/i/${ele._hash}' width=${ele._state.width} height=${ele._state.height}>
      <spinner-sk active></spinner-sk>
    </video>
    <figcaption>skottie</figcaption>
  </figure>
  <figure>
    <div id=container style='width: ${ele._state.width}px; height: ${ele._state.height}px'>
    </div>
    <figcaption>lottie-web (${bodymovin.version})</figcaption>
  </figure>
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
    // One of "dialog", "loading", or "loaded"
    this._ui = DIALOG_MODE;
    this._hash = '';
    this._anim = null;
    this._video = null;
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
      this._hash = '1112d01d28a776d777cebcd0632da15b';
    } else {
      this._hash = match[1];
    }
    this._ui = LOADING_MODE;
    this._render();
    fetch(`/_/j/${this._hash}`).then(jsonOrThrow).then(json => {
      this._state = json;
      this._ui = LOADED_MODE;
      this._render();
    }).catch((msg) => {
      msg.resp.text().then(errorMessage);
      window.history.pushState(null, '', '/');
      this._ui = DIALOG_MODE;
      this._render();
    });
  }

  _startEdit() {
    this._ui = DIALOG_MODE;
    this._render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._ui = LOADING_MODE;
    this._hash = '';
    this._render();
    fetch("/_/upload", {
      body: JSON.stringify(this._state),
      headers: {
        'content-type': 'application/json'
      },
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with the hash.
      this._ui = LOADED_MODE;
      this._hash = json.hash;
      window.history.pushState(null, '', '/' + this._hash);
      this._render();
    }).catch(msg => {
      this._ui = DIALOG_MODE;
      this._render();
      msg.resp.text().then(errorMessage);
    });
  }

  _playpause(e) {
    if (e.target.textContent == "Pause") {
      this._anim.pause();
      this._video.pause();
      e.target.textContent = "Play";
    } else {
      this._anim.play();
      this._video.play();
      e.target.textContent = "Pause";
    }
  }

  _rewind(e) {
    if ($$('#playpause', this).textContent == "Play") {
      this._anim.goToAndStop(0);
      this._video.currentTime = 0;
    } else {
      this._anim.goToAndPlay(0);
      this._video.currentTime = 0;
      this._video.play();
    }
  }

  _videoLoaded(e) {
    e.target.play();
    this._anim.play();
  }

  _render() {
    render(template(this), this);
    if (this._ui == LOADED_MODE) {
      this._anim = bodymovin.loadAnimation({
        container: $$('#container'),
        renderer: 'svg',
        loop: true,
        autoplay: false,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this._state.lottie)),
        rendererSettings: {
          preserveAspectRatio:'xMidYMid meet'
        },
      });
      this._video = $$('#video', this);
    }
  }

  handleEvent(e) {
    if (e.type == 'skottie-selected') {
      this._state = e.detail;
      this._upload();
    } else if (e.type == 'cancelled') {
      this._ui = LOADED_MODE;
      this._render();
    } else if (e.type == 'popstate') {
      this._reflectFromURL();
    }
  }

});
