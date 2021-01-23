/**
 * @module skottie-library-sk
 * @description <h2><code>skottie-library-sk</code></h2>
 *
 * <p>
 *   A skottie library selector
 * </p>
 *
 *
 * @evt select - This event is triggered when an animation is selected from the list.
 *
 * @attr animation - the seek position
 *
 */
import '../skottie-player-sk';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import JSZip from 'jszip';

const THUMBNAIL_SIZE = 100;

const animationTemplate = (ele, item, index) => html`
  <li
    class=thumbnail
  >
    <skottie-player-sk
      id=skottie_preview_${index}
      paused
      width=${THUMBNAIL_SIZE}
      height=${THUMBNAIL_SIZE}
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
      <label class=header-save-button>Upload zip
        <input
          type=file
          name=file
          id=file
          @change=${ele._onFileChange}
        />
      </label>
    </header>
    <section>
      <ul class="thumbnails">
         ${ele._state.animations.map((item, index) => animationTemplate(ele, item, index))}
      </ul>
    <section>
  </div>
`;

class SkottieLibrarySk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      animations: [],
      initialized: false,
    };
  }

  _onThumbSelected(item) {
    console.log('item', item);
    this.dispatchEvent(new CustomEvent('select', {
      detail: item,
    }));
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
          return '';
        }
      })
      .filter((animation) => animation);
    this._state.animations = parsedAnimations;
    this._state.initialized = false;
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }

  _initializePlayers() {
    if (!this._state.initialized) {
      this._state.initialized = true;
      this._state.animations.forEach((animation, index) => {
        const skottiePlayer = $$(`#skottie_preview_${index}`, this);
        skottiePlayer.initialize({
          width: THUMBNAIL_SIZE,
          height: THUMBNAIL_SIZE,
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
