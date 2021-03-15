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
const animationsPerPageOptions = [2, 5, 10, 15];
const INPUT_FILE_ID = 'fileInput';
const ITEMS_PER_PAGE_ID = 'libraryItemsPerPage';
const LIBRARY_PAGE_ID = 'libraryPage';
const THUMBNAIL_SIZE_ID = 'thumbnailSize';
const defaultAnimations = []; // TODO: preload provided animations

const animationTemplate = (ele, index) => html`
  <li
    id=skottie_preview_container_${index}
    class=thumbnail
  >
    <skottie-player-sk
      id=skottie_preview_${index}
      paused
      width=${ele._thumbnail_size}
      height=${ele._thumbnail_size}
      @click=${() => ele._onThumbSelected(index)}
    >
    </skottie-player-sk>
  </li>
`;

const buildPagesDropdown = (ele) => {
  const totalAnimationsCount = Math.ceil(
    ele._state.filesContent.length / ele._state.items_per_page,
  );
  // if there is less than two pages, skip the page renderer
  if (totalAnimationsCount <= 1) {
    return null;
  }
  const options = Array(totalAnimationsCount)
    .fill(0)
    .map((_, index) => html`
      <option
        value=${index}
        ?selected=${ele._state.current_page === index}
      >
        ${index + 1}
      </option>`);
  return html`
    <label class=page>
      Page
      <select id=${LIBRARY_PAGE_ID} class=dropdown>
      ${options}
      </select>
    </label>
  `;
};

const buildItemsPerPagesDropdown = (ele) => html`
  <label class=page>
    Animations per page
    <select id=${ITEMS_PER_PAGE_ID} class=dropdown>
      ${animationsPerPageOptions.map((item) => html`
        <option
          value=${item}
          ?selected=${ele._state.items_per_page === item}
        >${item}</option>
        `)}
  </label>
`;

const template = (ele) => html`
  <div>
    <header class="header">
      <div class="header-title">Skottie Library</div>
      <div class="header-separator"></div>
    </header>
    <section>
      ${buildPagesDropdown(ele)}
      <ul class=thumbnails>
        ${Array(ele._state.items_per_page).fill(0).map(
          (_, index) => animationTemplate(ele, index))
        }
      </ul>
      <div class=options>
        ${buildItemsPerPagesDropdown(ele)}
        <label class=header-save-button>Load zip
          <input
            type=file
            name=file
            id=${INPUT_FILE_ID}
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
            id=${THUMBNAIL_SIZE_ID}
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
      filesContent: [],
      initialized: false,
      items_per_page: animationsPerPageOptions[0],
      current_page: 0,
      texts: null,
    };
    this._syncAnimations = false;
    this._thumbnail_size = THUMBNAIL_SIZE;
  }

  _onThumbSelected(index) {
    this.dispatchEvent(new CustomEvent('select', {
      detail: this._state.animations[index],
    }));
    this._render();
  }

  _toggleSync(e) {
    // avoid double toggles
    e.preventDefault();
    this._syncAnimations = !this._syncAnimations;
    this._render();
  }

  _resetPage() {
    this._state.current_page = 0;
  }

  async _onFileChange(event) {
    const file = event.target.files[0];
    const content = await JSZip.loadAsync(file);
    this._state.filesContent = Object.keys(content.files)
      .map((key) => content.files[key]);
    this._state.current_page = 0;
    this._state.initialized = false;
  }

  _onPageChange() {
    this._state.initialized = false;
    this._state.current_page = parseInt($$('#libraryPage', this).value, 10);
  }

  _onThumbnailSizeChange(ev) {
    ev.preventDefault();
    this._thumbnail_size = ev.target.value;
    this._state.initialized = false;
    this._render();
  }

  connectedCallback() {
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  _updateState() {
    this._state.initialized = false;
    this._state.current_page = 0;
    const libraryItemsPerPage = $$('#libraryItemsPerPage', this);
    this._state.items_per_page = parseInt(libraryItemsPerPage.value, 10);
  }

  async _inputEvent(ev) {
    if (ev.target.id === INPUT_FILE_ID) {
      await this._onFileChange(ev);
    } else if (ev.target.id === LIBRARY_PAGE_ID) {
      await this._onPageChange(ev);
    } else if (ev.target.id === THUMBNAIL_SIZE_ID) {
      // we don't want to update the render every time the thumbnail size fires an input change
      return;
    } else {
      this._updateState();
    }
    this._render();
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
  }

  replaceTexts(texts) {
    this._state.initialized = false;
    this._state.texts = texts;
    this._state.animations = this._state.animations.map((animation) => replaceTextsByLayerName(texts, animation));
    this._render();
  }

  seek(frame) {
    if (this._syncAnimations) {
      Array(this._state.items_per_page)
        .fill(0)
        .forEach((_, index) => {
          const skottiePlayer = $$(`#skottie_preview_${index}`, this);
          skottiePlayer.seek(frame);
        });
    }
  }

  async _delay(time = 100) {
    return new Promise((resolve) => setTimeout(resolve, time));
  }

  _hidePlayers() {
    const itemsPerPage = this._state.items_per_page;
    let index = 0;
    while (index < itemsPerPage) {
      const skottiePlayerContainer = $$(`#skottie_preview_container_${index}`, this);
      skottiePlayerContainer.style.display = 'none';
      index += 1;
    }
  }

  async _initializePlayers() {
    if (!this._state.initialized) {
      this._hidePlayers();
      this._state.initialized = true;
      const currentFilesContent = this._state.filesContent;
      const page = this._state.current_page;
      const itemsPerPage = this._state.items_per_page;
      const texts = this._state.texts;
      let index = 0;
      while (index < itemsPerPage) {
        const currentAnimationIndex = itemsPerPage * page + index;
        if (
          currentFilesContent !== this._state.filesContent // if loaded animations have changed
          || page !== this._state.current_page // or page has changed
          || texts !== this._state.texts // or texts have changed
          || itemsPerPage !== this._state.items_per_page // or itemsPerPage have changed
          // or animation index exceeds total animations
          || currentAnimationIndex >= currentFilesContent.length
        ) {
          break; // we stop the async process
        }
        const animationFile = currentFilesContent[currentAnimationIndex];
        try {
          // eslint-disable-next-line no-await-in-loop
          const animation = await animationFile.async('text');
          const animationData = replaceTextsByLayerName(texts, JSON.parse(animation));
          this._state.animations[index] = animationData;
          const skottiePlayerContainer = $$(`#skottie_preview_container_${index}`, this);
          const skottiePlayer = $$(`#skottie_preview_${index}`, this);
          skottiePlayerContainer.style.display = 'inline-block';
          skottiePlayer.initialize({
            width: this._thumbnail_size,
            height: this._thumbnail_size,
            lottie: animationData,
            assets: [],
            fps: animationData.fr,
          });
        } catch (error) {
          console.error(error); // eslint-disable-line no-console
        }
        index += 1;
        await this._delay(); // eslint-disable-line no-await-in-loop
      }
    }
  }

  _render() {
    render(template(this), this, { eventContext: this });
    this._initializePlayers();
  }
}

define('skottie-library-sk', SkottieLibrarySk);
