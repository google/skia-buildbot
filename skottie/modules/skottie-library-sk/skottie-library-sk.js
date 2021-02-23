/**
 * @module skottie-library-sk
 * @description <h2><code>skottie-library-sk</code></h2>
 *
 * <p>
 *   A skottie library selector.
     It allows users to upload a zip file of animations (containing a collection of lottie jsons).
     For each animation, it makes an animation player.
     This can be useful for quickly comparing animations, viewing them in sync
     or test new texts on all the same time.
 * </p>
 *
 *
 * @evt select - This event is triggered when an animation is selected from the list.
 *
 *
 */
import '../skottie-player-sk';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import JSZip from 'jszip';
import { replaceTextsByLayerName } from '../skottie-text-editor/text-replace';

const THUMBNAIL_SIZE = 200;
const defaultAnimations = []; // TODO: preload provided animations

const animationTemplate = (ele, item, index) => html`
  <li
    class=thumbnail
  >
    <skottie-player-sk
      id=skottie_preview_${index}
      paused
      width=${ele._thumbnail_size}
      height=${ele._thumbnail_size}
      @click=${() => ele._onThumbSelected(item)}
    >
    </skottie-player-sk>
  </li>
`;

const template = (ele) => html`
  <div>
    <header class="header">
      <div class="header-title">Skottie Library</div>
      <div class="header-separator"></div>
    </header>
    <section>
      <ul class=thumbnails>
         ${ele._state.animations.map((item, index) => animationTemplate(ele, item, index))}
      </ul>
      <div class=options>
      <label class=header-save-button>Load zip
        <input
          type=file
          name=file
          id=file
          @change=${ele._onFileChange}
        />
      </label>
      <checkbox-sk
        label="Sync thumbnails"
        ?checked=${ele._syncAnimations}
        @click=${ele._toggleSync}>
      </checkbox-sk>
      <label class=size>
        <input
          type=number
          id=thumbnail_size
          .value=${ele._thumbnail_size}
          @change=${ele._onThumbnailSizeChange}
          required
        /> Thumbnail Size (px)
      </label>
      </div>
    <section>
  </div>
`;

class SkottieLibrarySk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      animations: defaultAnimations,
      initialized: false,
    };
    this._syncAnimations = false;
    this._thumbnail_size = THUMBNAIL_SIZE;
  }

  _onThumbSelected(item) {
    this.dispatchEvent(new CustomEvent('select', {
      detail: item,
    }));
    this._render();
  }

  _toggleSync(e) {
    // avoid double toggles
    e.preventDefault();
    this._syncAnimations = !this._syncAnimations;
    this._render();
  }

  async _onFileChange(event) {
    const file = event.target.files[0];
    const content = await JSZip.loadAsync(file);
    const animations = await Promise.all(
      Object.keys(content.files)
        .map((key) => content.files[key].async('text')),
    );
    const parsedAnimations = animations
      .map((animation) => {
        try {
          return JSON.parse(animation);
        } catch (error) {
          console.log(error); // eslint-disable-line no-console
          return '';
        }
      })
      .filter((animation) => animation);
    this._state.animations = parsedAnimations;
    this._state.initialized = false;
    this._render();
  }

  _onThumbnailSizeChange(ev) {
    ev.preventDefault();
    this._thumbnail_size = ev.target.value;
    this._state.initialized = false;
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }

  replaceTexts(texts) {
    this._state.initialized = false;
    this._state.animations = this._state.animations.map((animation) => replaceTextsByLayerName(texts, animation));
    this._render();
  }

  seek(frame) {
    if (this._syncAnimations) {
      this._state.animations.forEach((animation, index) => {
        const skottiePlayer = $$(`#skottie_preview_${index}`, this);
        skottiePlayer.seek(frame);
      });
    }
  }

  _initializePlayers() {
    if (!this._state.initialized) {
      this._state.initialized = true;
      this._state.animations.forEach((animation, index) => {
        const skottiePlayer = $$(`#skottie_preview_${index}`, this);
        skottiePlayer.initialize({
          width: this._thumbnail_size,
          height: this._thumbnail_size,
          lottie: animation,
          assets: [],
          fps: animation.fr,
        });
      });
    }
  }

  _render() {
    render(template(this), this, { eventContext: this });
    this._initializePlayers();
  }
}

define('skottie-library-sk', SkottieLibrarySk);
