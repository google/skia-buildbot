/**
 * @module module/image-compare-sk
 * @description <h2><code>image-compare-sk</code></h2>
 *
 * Shows a side by side comparison of two images. If there's nothing to compare against, it will
 * only display one.
 *
 * @event zoom-dialog-opened when the user opens the multi-zoom-sk dialog.
 *
 * @event zoom-dialog-closed when the user closes the multi-zoom-sk dialog.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/open-in-new-icon-sk';
import 'elements-sk/styles/buttons';
import { digestDiffImagePath, digestImagePath } from '../common';

import '../multi-zoom-sk';

const template = (ele) => html`
<div class=comparison_bar>
  <figure>
    <img class=thumbnail alt="left image" src=${digestImagePath(ele.left.digest)}>
    <figcaption>
      <span class=legend_dot></span>
      <a target=_blank rel=noopener href=${ele.left.detail}>${ele.left.title}</a>
    </figcaption>
  </figure>
  <a target=_blank rel=noopener href=${digestImagePath(ele.left.digest)}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
  ${comparison(ele)}
</div>

<button class=zoom_btn ?hidden=${!ele.right} @click=${ele._handleZoomClicked}>Zoom</button>
<dialog class=zoom_dialog @close=${ele._closeEvent}))}>
  <button class=close_btn @click=${ele._closeDialog}>Close</button>
</dialog>
`;

const comparison = (ele) => {
  if (!ele.right ) {
    if (ele.isComputingDiffs) {
      return html`<div class=computing title="Check back later">
            Computing closest positive and negative image. Check back later.</div>`;
    }
    return html`<div class=no_compare>No other images to compare against.</div>`;
  }
  const diffSrc = digestDiffImagePath(ele.left.digest, ele.right.digest);
  return html`
<img class="thumbnail diff" alt="diff between left and right image" src=${diffSrc}>
<a target=_blank rel=noopener href=${diffSrc}>
  <open-in-new-icon-sk></open-in-new-icon-sk>
</a>

<figure>
  <img class=thumbnail alt="right image" src=${digestImagePath(ele.right.digest)}>
  <figcaption>
    <a target=_blank rel=noopener href=${ele.right.detail}>${ele.right.title}</a>
  </figcaption>
</figure>
<a target=_blank rel=noopener href=${digestImagePath(ele.right.digest)}>
  <open-in-new-icon-sk></open-in-new-icon-sk>
</a>`;
};

define('image-compare-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._left = {
      digest: '',
      title: '',
      // We can't derive the detail url w/o also passing down changelistID, crs etc, so we have
      // the caller compute those URLs and pass them into this element.
      detail: '',
    };
    this._right = null;
    this._computingDiffs = false;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    dialogPolyfill.registerDialog($$('dialog.zoom_dialog', this));
  }

  get isComputingDiffs() {return this._computingDiffs; }

  set isComputingDiffs(b) {
    this._computingDiffs = !!b;
    this._render();
  }

  /**
   * @prop left {object} An object with string properties digest, title, detail.
   */
  get left() { return this._left; }

  set left(obj) {
    this._left = obj;
    this._render();
  }

  /**
   * @prop right {object} An object with string properties digest, title, detail.
   */
  get right() { return this._right; }

  set right(obj) {
    this._right = obj;
    this._render();
  }

  _closeDialog() {
    const dialog = $$('dialog.zoom_dialog', this);
    if (dialog) {
      dialog.close(); // this will fire a close event
    }
  }

  _closeEvent() {
    // We clean up both when the user clicks the close button as well if they hit escape by waiting
    // for the close event (instead of handling this in _closeDialog().
    const dialog = $$('dialog.zoom_dialog', this);
    const zoom = $$('dialog multi-zoom-sk', this);
    if (dialog && zoom) {
      // Removing the element from the dom removes the keybinding handlers and lets the browser
      // free up the image resources.
      dialog.removeChild(zoom);
    }
    this.dispatchEvent(new CustomEvent('zoom-dialog-closed', { bubbles: true }));
  }

  _handleZoomClicked() {
    const ele = document.createElement('multi-zoom-sk');
    ele.details = {
      leftImageSrc: digestImagePath(this.left.digest),
      rightImageSrc: digestImagePath(this.right.digest),
      diffImageSrc: digestDiffImagePath(this.left.digest, this.right.digest),
      leftLabel: this.left.title,
      rightLabel: this.right.title,
    };
    const dialog = $$('dialog.zoom_dialog', this);
    // put the dialog before the button
    dialog.insertBefore(ele, dialog.childNodes[0]);
    dialog.showModal();
    this.dispatchEvent(new CustomEvent('zoom-dialog-opened', { bubbles: true }));
  }
});
