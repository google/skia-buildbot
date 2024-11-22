/**
 * @module module/image-compare-sk
 * @description <h2><code>image-compare-sk</code></h2>
 *
 * Shows a side by side comparison of two images. If there's nothing to compare against, it will
 * only display one.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { MultiZoomSk } from '../multi-zoom-sk/multi-zoom-sk';

import '../../../elements-sk/modules/icons/open-in-new-icon-sk';
import { digestDiffImagePath, digestImagePath } from '../common';

import '../multi-zoom-sk';

export interface ImageComparisonData {
  digest: string;
  title: string;
  detail: string;
}

export class ImageCompareSk extends ElementSk {
  private static template = (ele: ImageCompareSk) => html`
    <div class="comparison_bar">
      <figure>
        <img
          class="thumbnail ${ele._fullSizeLeftImage ? 'fullsize' : ''}"
          alt="left image"
          src=${digestImagePath(ele.left.digest)}
          @click=${ele.toggleFullSizeLeftImage} />
        <figcaption>
          <span class="legend_dot"></span>
          <a target="_blank" rel="noopener" href=${ele.left.detail}
            >${ele.left.title}</a
          >
        </figcaption>
      </figure>
      <a
        target="_blank"
        rel="noopener"
        href=${digestImagePath(ele.left.digest)}>
        <open-in-new-icon-sk></open-in-new-icon-sk>
      </a>
      ${ImageCompareSk.comparison(ele)}
    </div>

    <button class="zoom_btn" ?hidden=${!ele.right} @click=${ele.openZoomWindow}>
      Zoom
    </button>
    <dialog class="zoom_dialog" @close="${ele.closeEvent}))}">
      <button class="close_btn" @click=${ele.closeDialog}>Close</button>
    </dialog>
  `;

  private static comparison = (ele: ImageCompareSk) => {
    if (!ele.right) {
      if (ele.isComputingDiffs) {
        return html`<div class="computing" title="Check back later">
          Computing closest positive and negative image. Check back later.
        </div>`;
      }
      return html`<div class="no_compare">
        No other images to compare against.
      </div>`;
    }
    const diffSrc = digestDiffImagePath(ele.left.digest, ele.right.digest);
    return html`
      <img
        class="thumbnail diff ${ele._fullSizeDiffImage ? 'fullsize' : ''}"
        alt="diff between left and right image"
        src=${diffSrc}
        @click=${ele.toggleFullSizeDiffImage} />
      <a target="_blank" rel="noopener" href=${diffSrc}>
        <open-in-new-icon-sk></open-in-new-icon-sk>
      </a>

      <figure>
        <img
          class="thumbnail ${ele._fullSizeRightImage ? 'fullsize' : ''}"
          alt="right image"
          src=${digestImagePath(ele.right.digest)}
          @click=${ele.toggleFullSizeRightImage} />
        <figcaption>
          <a target="_blank" rel="noopener" href=${ele.right.detail}
            >${ele.right.title}</a
          >
        </figcaption>
      </figure>
      <a
        target="_blank"
        rel="noopener"
        href=${digestImagePath(ele.right.digest)}>
        <open-in-new-icon-sk></open-in-new-icon-sk>
      </a>
    `;
  };

  private _left: ImageComparisonData = {
    digest: '',
    title: '',
    // We can't derive the detail url w/o also passing down changelistID, crs etc, so we have
    // the caller compute those URLs and pass them into this element.
    detail: '',
  };

  private _right: ImageComparisonData | null = null;

  private computingDiffs = false;

  private _fullSizeImages = false;

  private _fullSizeLeftImage = false;

  private _fullSizeDiffImage = false;

  private _fullSizeRightImage = false;

  constructor() {
    super(ImageCompareSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  get isComputingDiffs(): boolean {
    return this.computingDiffs;
  }

  set isComputingDiffs(b: boolean) {
    this.computingDiffs = b;
    this._render();
  }

  get left(): ImageComparisonData {
    return this._left;
  }

  set left(img: ImageComparisonData) {
    this._left = img;
    this._render();
  }

  get right(): ImageComparisonData | null {
    return this._right;
  }

  set right(img: ImageComparisonData | null) {
    this._right = img;
    this._render();
  }

  /** Whether to show thumbnails or full size images. */
  get fullSizeImages(): boolean {
    return this._fullSizeImages;
  }

  set fullSizeImages(val: boolean) {
    this._fullSizeImages = val;
    this._fullSizeLeftImage = val;
    this._fullSizeDiffImage = val;
    this._fullSizeRightImage = val;
    this._render();
  }

  private closeDialog() {
    // This will fire a close event.
    this.querySelector<HTMLDialogElement>('dialog.zoom_dialog')?.close();
  }

  private closeEvent() {
    // We clean up both when the user clicks the close button as well if they hit escape by waiting
    // for the close event (instead of handling this in closeDialog().
    const dialog = this.querySelector<HTMLDialogElement>('dialog.zoom_dialog');
    const zoom = this.querySelector<MultiZoomSk>('dialog multi-zoom-sk');
    if (dialog && zoom) {
      // Removing the element from the dom removes the keybinding handlers and lets the browser
      // free up the image resources.
      dialog.removeChild(zoom);
    }
  }

  openZoomWindow(): void {
    const ele = new MultiZoomSk();
    ele.details = {
      leftImageSrc: digestImagePath(this.left.digest),
      rightImageSrc: digestImagePath(this.right!.digest),
      diffImageSrc: digestDiffImagePath(this.left.digest, this.right!.digest),
      leftLabel: this.left.title,
      rightLabel: this.right!.title,
    };
    const dialog = this.querySelector<HTMLDialogElement>('dialog.zoom_dialog')!;
    // put the dialog before the button
    dialog.insertBefore(ele, dialog.childNodes[0]);
    dialog.showModal();
  }

  private toggleFullSizeLeftImage(): void {
    this._fullSizeLeftImage = !this._fullSizeLeftImage;
    this._render();
    this._dispatchImageSizeToggledEvent();
  }

  private toggleFullSizeDiffImage(): void {
    this._fullSizeDiffImage = !this._fullSizeDiffImage;
    this._render();
    this._dispatchImageSizeToggledEvent();
  }

  private toggleFullSizeRightImage(): void {
    this._fullSizeRightImage = !this._fullSizeRightImage;
    this._render();
    this._dispatchImageSizeToggledEvent();
  }

  private _dispatchImageSizeToggledEvent(): void {
    this.dispatchEvent(
      new CustomEvent('image_compare_size_toggled', { bubbles: true })
    );
  }
}

define('image-compare-sk', ImageCompareSk);
