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
import { html } from 'lit-html';
import JSZip, { JSZipObject } from 'jszip';
import { replaceTextsByLayerName, TextData } from '../skottie-text-editor-sk/text-replace';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';

const THUMBNAIL_SIZE = 200;
const animationsPerPageOptions = [2, 5, 10, 15];
const INPUT_FILE_ID = 'fileInput';
const ITEMS_PER_PAGE_ID = 'libraryItemsPerPage';
const LIBRARY_PAGE_ID = 'libraryPage';
const THUMBNAIL_SIZE_ID = 'thumbnailSize';

export class SkottieLibrarySk extends ElementSk {
  private static template = (ele: SkottieLibrarySk) => html`
  <div>
    <header class="header">
      <div class="header-title">Skottie Library</div>
      <div class="header-separator"></div>
    </header>
    <section>
      ${ele.buildPagesDropdown()}
      <ul class=thumbnails>
        ${Array(ele.itemsPerPage).fill(0).map(
    (_, index: number) => ele.animationTemplate(index),
  )
  }
      </ul>
      <div class=options>
        ${ele.buildItemsPerPagesDropdown()}
        <label class=header-save-button>Load zip
          <input
            type=file
            name=file
            id=${INPUT_FILE_ID}
          />
        </label>
        <checkbox-sk
          label="Sync thumbnails"
          title="If selected, all animations will play at the same time as the main animation.
If not selected, the animations will be paused and not respond to scrubbing of the timeline."
          ?checked=${ele.syncAnimations}
          @click=${ele.toggleSync}>
        </checkbox-sk>
        <label class=size>
          <input
            type=number
            id=${THUMBNAIL_SIZE_ID}
            .value=${ele.thumbnailSize}
            @change=${ele.onThumbnailSizeChange}
            required
          /> Thumbnail Size (px)
        </label>
      </div>
    <section>
  </div>
`;

  private buildItemsPerPagesDropdown = () => html`
  <label class=page>
    Animations per page
    <select id=${ITEMS_PER_PAGE_ID} class=dropdown>
      ${animationsPerPageOptions.map((item) => html`
        <option
          value=${item}
          ?selected=${this.itemsPerPage === item}
        >${item}</option>
        `)}
  </label>
`;

  private buildPagesDropdown = () => {
    const totalAnimationsCount = Math.ceil(
      this.filesContent.length / this.itemsPerPage,
    );
    // if there is less than two pages, skip the page renderer
    if (totalAnimationsCount <= 1) {
      return null;
    }
    const options = [];
    for (let i = 0; i < totalAnimationsCount; i++) {
      options.push(html`
      <option
        value=${i}
        ?selected=${this.currentPage === i}
      >
        ${i + 1}
      </option>`);
    }
    return html`
    <label class=page>
      Page
      <select id=${LIBRARY_PAGE_ID} class=dropdown>
      ${options}
      </select>
    </label>
  `;
  };

  private animationTemplate = (index: number) => html`
  <li
    id=skottie_preview_container_${index}
    class=thumbnail
  >
    <skottie-player-sk
      id=skottie_preview_${index}
      paused
      width=${this.thumbnailSize}
      height=${this.thumbnailSize}
      @click=${() => this.onThumbSelected(index)}
    >
    </skottie-player-sk>
  </li>
`;

  private animations: LottieAnimation[] = [];

  private currentPage: number = 0;

  private filesContent: JSZipObject[] = [];

  private initialized: boolean = false;

  private itemsPerPage: number = animationsPerPageOptions[0];

  private syncAnimations: boolean = false;

  private texts: TextData[] = [];

  private thumbnailSize: number = THUMBNAIL_SIZE;

  constructor() {
    super(SkottieLibrarySk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback(): void {
    this.removeEventListener('input', this._inputEvent);
    super.disconnectedCallback();
  }

  private onThumbSelected(index: number): void {
    this.dispatchEvent(new CustomEvent<LottieAnimation>('select', {
      detail: this.animations[index],
    }));
    this._render();
  }

  private toggleSync(e: Event) {
    // avoid double toggles
    e.preventDefault();
    this.syncAnimations = !this.syncAnimations;
    this._render();
  }

  private async onFileChange(e: Event): Promise<void> {
    const file = (e.target as HTMLInputElement).files![0];
    const content = await JSZip.loadAsync(file);
    this.filesContent = [];
    for (const fileName of Object.keys(content.files)) {
      // Without this, we'll try to render the .JSON files in there, which are not valid.
      if (fileName.startsWith('__MACOS')) {
        continue;
      }
      this.filesContent.push(content.files[fileName]);
    }
    this.filesContent.sort((a: JSZipObject, b: JSZipObject) => a.name.localeCompare(b.name));
    this.currentPage = 0;
    this.initialized = false;
  }

  private onPageChange(): void {
    this.initialized = false;
    this.currentPage = parseInt($$<HTMLSelectElement>('#libraryPage', this)!.value, 10);
  }

  private onThumbnailSizeChange(e: Event): void {
    e.preventDefault();
    this.thumbnailSize = +(e.target as HTMLInputElement).value;
    this.initialized = false;
    this._render();
  }

  private updateState(): void {
    this.initialized = false;
    this.currentPage = 0;
    const libraryItemsPerPage = $$<HTMLSelectElement>('#libraryItemsPerPage', this)!;
    this.itemsPerPage = parseInt(libraryItemsPerPage.value, 10);
  }

  private async _inputEvent(e: Event): Promise<void> {
    const id = (e.target! as HTMLElement).id;
    if (id === INPUT_FILE_ID) {
      await this.onFileChange(e);
    } else if (id === LIBRARY_PAGE_ID) {
      this.onPageChange();
    } else if (id === THUMBNAIL_SIZE_ID) {
      // we don't want to update the render every time the thumbnail size fires an input change
      return;
    } else {
      this.updateState();
    }
    this._render();
  }

  replaceTexts(texts: TextData[]): void {
    this.initialized = false;
    this.texts = texts;
    this.animations = this.animations.map((animation: LottieAnimation) => replaceTextsByLayerName(texts, animation));
    this._render();
  }

  seek(frame: number): void {
    if (this.syncAnimations) {
      for (let i = 0; i < this.itemsPerPage; i++) {
        const skottiePlayer = $$<SkottiePlayerSk>(`#skottie_preview_${i}`, this);
        if (skottiePlayer) {
          skottiePlayer.seek(frame);
        }
      }
    }
  }

  private hidePlayers(): void {
    for (let i = 0; i < this.itemsPerPage; i++) {
      const skottiePlayerContainer = $$<HTMLLIElement>(`#skottie_preview_container_${i}`, this)!;
      skottiePlayerContainer.style.display = 'none';
    }
  }

  private async initializePlayers(): Promise<void> {
    if (!this.initialized) {
      this.hidePlayers();
      this.initialized = true;
      const currentFilesContent = this.filesContent;
      const page = this.currentPage;
      const itemsPerPage = this.itemsPerPage;
      const texts = this.texts;
      for (let i = 0; i < itemsPerPage; i++) {
        const currentAnimationIndex = itemsPerPage * page + i;
        if (
          currentFilesContent !== this.filesContent // if loaded animations have changed
          || page !== this.currentPage // or page has changed
          || texts !== this.texts // or texts have changed
          || itemsPerPage !== this.itemsPerPage // or itemsPerPage have changed
          // or animation index exceeds total animations
          || currentAnimationIndex >= currentFilesContent.length
        ) {
          break; // we stop the async process
        }
        const animationFile = currentFilesContent[currentAnimationIndex];
        try {
          // eslint-disable-next-line no-await-in-loop
          const animation = await animationFile.async('text');
          const animationData = replaceTextsByLayerName(texts, JSON.parse(animation) as LottieAnimation);
          animationData.metadata = {
            ...animationData.metadata,
            filename: animationFile.name,
          };
          this.animations[i] = animationData;
          const skottiePlayerContainer = $$<HTMLLIElement>(`#skottie_preview_container_${i}`, this)!;
          const skottiePlayer = $$<SkottiePlayerSk>(`#skottie_preview_${i}`, this)!;
          skottiePlayerContainer.style.display = 'inline-block';
          // eslint-disable-next-line no-await-in-loop
          await skottiePlayer.initialize({
            width: this.thumbnailSize,
            height: this.thumbnailSize,
            lottie: animationData,
            fps: animationData.fr as number,
          });
        } catch (error) {
          console.error(error); // eslint-disable-line no-console
        }
      }
    }
  }

  _render(): void {
    super._render();
    this.initializePlayers();
  }
}

define('skottie-library-sk', SkottieLibrarySk);
