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
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { setupListeners, onUserEdit, reannotate} from '../lottie-annotations'
import { stateReflector } from 'common-sk/modules/stateReflector'
import '../skottie-text-editor'

const JSONEditor = require('jsoneditor/dist/jsoneditor-minimalist.js');
const bodymovin = require('lottie-web/build/player/lottie.min.js');

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;

const GOOGLE_WEB_FONTS_HOST = 'https://storage.googleapis.com/skia-cdn/google-web-fonts';

// SCRUBBER_RANGE is the input range for the scrubbing control.
// This is an arbitrary value, and is treated as a re-scaled duration.
const SCRUBBER_RANGE = 1000;

const displayDialog = (ele) => html`
<skottie-config-sk .state=${ele._state} .width=${ele._width}
    .height=${ele._height} .fps=${ele._fps}></skottie-config-sk>
`;

const skottiePlayer = (ele) => html`
<skottie-player-sk paused width=${ele._width} height=${ele._height}>
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
       style='width: ${ele._width}px; height: ${ele._height}px'></div>
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
       style='width: ${ele._width}px; height: ${ele._height}px'></div>
  <figcaption>Preview [lottie-web]</figcaption>
</figure>`;
  }
}

const iframeDirections = (ele) => {
  return `<iframe width="${ele._width}" height="${ele._height}" src="${window.location.origin}/e/${ele._hash}?w=${ele._width}&h=${ele._height}" scrolling=no>`;
}

const inlineDirections = (ele) => {
  return `<skottie-inline-sk width="${ele._width}" height="${ele._height}" src="${window.location.origin}/_/j/${ele._hash}"></skottie-inline-sk>`;
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

const jsonTextEditor = (ele) => {
  if (!ele._showTextEditor) {
    return '';
  }
  return html`
<section class=editor>
  <skottie-text-editor
    .animation=${ele._state.lottie}
    @apply=${ele._applyTextEdits}
  >
  </skottie-text-editor>
</section>`;
}

const displayLoaded = (ele) => html`
<button class=edit-config @click=${ ele._startEdit}>
  ${ele._state.filename} ${ele._width}x${ele._height} ...
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
  <checkbox-sk label="Show text editor"
               ?checked=${ele._showTextEditor}
               @click=${ele._toggleTextEditor}>
  </checkbox-sk>
  <button @click=${ele._toggleEmbed}>Embed</button>
  <div class=scrub>
    <input id=scrub type=range min=0 max=${SCRUBBER_RANGE+1} step=0.1
        @input=${ele._onScrub} @change=${ele._onScrubEnd}>
  </div>
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
${jsonTextEditor(ele)}
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
    return html`
<div>
  Googlers should use <a href="https://skottie-internal.skia.org">skottie-internal.skia.org</a>.
</div>`;
  } else {
    return html``;
  }
};

const template = (ele) => html`
<header>
  <h2>Skottie</h2>
  <span>
    <a href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'>
      ${SKIA_VERSION.slice(0, 7)}
    </a>
  </span>
</header>
<main>
  ${pick(ele)}
</main>
<footer>
  <error-toast-sk></error-toast-sk>
  ${redir(ele)}
</footer>
`;

define('skottie-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: null,
      assetsZip: '',
      assetsFilename: '',
    };
    // One of 'dialog', 'loading', or 'loaded'
    this._ui = DIALOG_MODE;
    this._hash = '';
    this._skottiePlayer = null;
    this._lottie = null;
    this._live = null;
    this._playing = true;
    this._assetsPath = '/_/a';
    this._downloadUrl = null; // The URL to download the lottie JSON from.
    this._editor = null;
    this._editorLoaded = false;
    this._hasEdits = false;
    this._showLottie = false;
    this._showEditor = false;
    this._showTextEditor = false;
    this._scrubbing = false;
    this._playingOnStartOfScrub = false;

    this._width = 0;
    this._height = 0;
    this._fps = 0;

    this._stateChanged = stateReflector(
      /*getState*/() => {
        return {
          // provide empty values
          'l' : this._showLottie,
          'e' : this._showEditor,
          't' : this._showTextEditor,
          'w' : this._width,
          'h' : this._height,
          'f' : this._fps,
        }
    }, /*setState*/(newState) => {
      this._showLottie = newState.l;
      this._showEditor = newState.e;
      this._showTextEditor = newState.t;
      this._width = newState.w;
      this._height = newState.h;
      this._fps = newState.f;
      this._applyTextEdits = this._applyTextEdits.bind(this);
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

        // lottie player takes the milliseconds from the beginning of the animation.
        this._lottie && this._lottie.goToAndStop(progress);
        this._live && this._live.goToAndStop(progress);
        const scrubber = $$('#scrub', this);
        if (scrubber) {
          // Scale from time to the arbitrary scrubber range.
          scrubber.value = SCRUBBER_RANGE * progress / this._duration;
        }
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

  _applyTextEdits(event) {
    const animation = event.detail;
    this._state.lottie = animation;
    this._upload();
  }

  _applyEdits() {
    if (!this._editor || !this._editorLoaded || !this._hasEdits) {
      return;
    }
    this._state.lottie = this._editor.get();
    this._upload();
  }

  _autoSize() {
    let changed = false;
    if (!this._width) {
      this._width = this._state.lottie.w;
      changed = true;
    }
    if (!this._height) {
      this._height = this._state.lottie.h;
      changed = true;
    }
    // By default, leave FPS at 0, instead of reading them from the lottie,
    // because that will cause it to render as smoothly as possible,
    // which looks better in most cases. If a user gives a negative value
    // for fps (e.g. -1), then we use either what the lottie tells us or
    // as fast as possible.
    if (this._fps < 0) {
      this._fps = this._state.lottie.fr || 0;
    }
    return changed;
  }

  handleEvent(e) {
    if (e.type === 'skottie-selected') {
      this._state = e.detail.state;
      this._width = e.detail.width;
      this._height = e.detail.height;
      this._fps = e.detail.fps;
      this._autoSize();
      this._stateChanged();
      if (e.detail.fileChanged) {
        this._upload();
      } else {
        this._ui = LOADED_MODE;
        this.render();
        this._initializePlayer();
        // Re-sync all players
        this._rewind();
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
    this._skottiePlayer.initialize({
      width:  this._width,
      height: this._height,
      lottie: this._state.lottie,
      assets: this._state.assets,
      fps:    this._fps,
    }).then(() => {
      this._duration = this._skottiePlayer.duration();
      // If the user has specified a value for FPS, we want to lock the
      // size of the scrubber so it is as discrete as the frame rate.
      if (this._fps) {
        const scrubber = $$("#scrub", this);
        if (scrubber) {
          // calculate a scaled version of ms per frame as the step size.
          scrubber.step = (1000 / this._fps * SCRUBBER_RANGE / this._duration);
        }
      }

    });
  }

  _loadAssetsAndRender() {
    const toLoad = [];

    const lottie = this._state.lottie;
    let fonts  = [];
    let assets = [];
    if (lottie.fonts && lottie.fonts.list) {
      fonts = lottie.fonts.list;
    }
    if (lottie.assets && lottie.assets.length) {
      assets = lottie.assets;
    }

    toLoad.push(...this._loadFonts(fonts));
    toLoad.push(...this._loadAssets(assets));

    Promise.all(toLoad).then((externalAssets) => {
      const assets = {};
      for (const asset of externalAssets) {
        if (asset) {
          assets[asset.name] = asset.bytes;
        }
      }

      // check fonts
      fonts.forEach(font => {
        if (!assets[font.fName]) {
          console.error(`Could not load font '${font.fName}'.`);
        }
      });

      this._state.assets = assets;
      this.render();
      this._initializePlayer();
      // Re-sync all players
      this._rewind();
    })
    .catch(() => {
      this.render();
      this._initializePlayer();
      // Re-sync all players
      this._rewind();
    });

  }

  _loadFonts(fonts) {
    const promises = [];
    for (const font of fonts) {
      if (!font.fName) {
        continue;
      }

      const fetchFont = (fontURL) => {
        promises.push(fetch(fontURL)
          .then((resp) => {
            // fetch does not reject on 404
            if (!resp.ok) {
              return null;
            }
            return resp.arrayBuffer().then((buffer) => {
              return {
                'name': font.fName,
                'bytes': buffer
              };
            });
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
      fetchFont(`${this._assetsPath}/${this._hash}/${font.fName}.ttf`);
    }

    return promises;
  }

  _loadAssets(assets) {
    const promises = [];
    for (const asset of assets) {
      // asset.p is the filename, if it's an image.
      // Don't try to load inline/dataURI images.
      const should_load = asset.p && asset.p.startsWith && !asset.p.startsWith('data:');
      if (should_load) {
        promises.push(fetch(`${this._assetsPath}/${this._hash}/${asset.p}`)
          .then((resp) => {
            // fetch does not reject on 404
            if (!resp.ok) {
              console.error(`Could not load ${asset.p}: status ${resp.status}`)
              return null;
            }
            return resp.arrayBuffer().then((buffer) => {
              return {
                'name': asset.p,
                'bytes': buffer
              };
            });
          })
        );
      }
    }
    return promises;
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

  _recoverFromError(msg) {
      errorMessage(msg);
      console.error(msg);
      window.history.pushState(null, '', '/');
      this._ui = DIALOG_MODE;
      this.render();
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
        // remove legacy fields from state, if they are there.
        delete this._state.width;
        delete this._state.height;
        delete this._state.fps;

        if (this._autoSize()) {
          this._stateChanged();
        }
        this._ui = LOADED_MODE;
        this._loadAssetsAndRender();
      }).catch((msg) => this._recoverFromError(msg));
    });
  }

  render() {
    if (this._downloadUrl)  {
      URL.revokeObjectURL(this._downloadUrl);
    }
    this._downloadUrl = URL.createObjectURL(new Blob([JSON.stringify(this._state.lottie)]));
    render(template(this), this, {eventContext: this});

    this._skottiePlayer = $$('skottie-player-sk', this);

    if (this._ui === LOADED_MODE) {
      try {
        this._renderLottieWeb();
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
      return;
    }
    let editorContainer = $$('#json_editor');
    // See https://github.com/josdejong/jsoneditor/blob/master/docs/api.md
    // for documentation on this editor.
    let editorOptions = {
      // Use original key order (this preserves related fields locality).
      sortObjectKeys: false,
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
        assetsPath: `${this._assetsPath}/${this._hash}/`,
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
        assetsPath: `${this._assetsPath}/${this._hash}/`,
        // Apparently the lottie player modifies the data as it runs?
        animationData: JSON.parse(JSON.stringify(this._editor.get())),
        rendererSettings: {
          preserveAspectRatio:'xMidYMid meet'
        },
      });
    }
  }

  // This fires every time the user moves the scrub slider.
  _onScrub(e) {
    if (!this._scrubbing) {
      // Pause the animation while dragging the slider.
      this._playingOnStartOfScrub = this._playing;
      if (this._playing) {
        this._playpause()
      }
      this._scrubbing = true;
    }

    let seek = (e.currentTarget.value / SCRUBBER_RANGE);
    this._live && this._live.goToAndStop(seek);
    this._lottie && this._lottie.goToAndStop(seek * this._duration);
    this._skottiePlayer && this._skottiePlayer.seek(seek);
  }

  // This fires when the user releases the scrub slider.
  _onScrubEnd(e) {
    if (this._playingOnStartOfScrub) {
      this._playpause()
    }
    this._scrubbing = false;
  }

  _rewind(e) {
    // Handle rewinding when paused.
    this._wasmTimePassed = 0;
    if (!this._playing) {
      this._skottiePlayer.seek(0);
      this._firstFrameTime = null;
      this._live && this._live.goToAndStop(0);
      this._lottie && this._lottie.goToAndStop(0);
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
    this._showTextEditor = false;
    this._showEditor = !this._showEditor;
    this._stateChanged();
    this.render();
  }

  _toggleTextEditor(e) {
    e.preventDefault();
    this._showEditor = false;
    this._showTextEditor = !this._showTextEditor;
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
    // POST the JSON to /_/upload
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
    }).then(jsonOrThrow).then((json) => {
      // Should return with the hash and the lottie file
      this._ui = LOADED_MODE;
      this._hash = json.hash;
      this._state.lottie = json.lottie;
      window.history.pushState(null, '', '/' + this._hash);
      this._stateChanged();
      if (this._state.assetsZip) {
        this._loadAssetsAndRender();
      }
      this.render();
    }).catch((msg) => this._recoverFromError(msg));

    if (!this._state.assetsZip) {
      this._ui = LOADED_MODE;
      // Start drawing right away, no need to wait for
      // the JSON to make a round-trip to the server, since there
      // are no assets that we need to unzip server-side.
      // We still need to check for things like webfonts.
      this.render();
      this._loadAssetsAndRender();
    } else {
      // We have to wait for the server to process the zip file.
      this._ui = LOADING_MODE;
      this.render();
    }

  }

});
