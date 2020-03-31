/**
 * @module module/image-compare-sk
 * @description <h2><code>image-compare-sk</code></h2>
 *
 * Shows a side by side comparison of two images. If there's nothing to compare against, it will
 * only display one.
 *
 * @evt zoom-clicked - When the zoom button is clicked, this event is produced. It has a detail
 *   object with the following string properties: leftImgUrl, rightImgUrl, middleImgUrl, llabel,
 *   rlabel. TODO(kjlubick) can we have this element have/show the dialog and get rid of the event?
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/open-in-new-icon-sk';
import 'elements-sk/styles/buttons';
import { digestDiffImagePath, digestImagePath } from '../common';

const template = (ele) => html`
<div class=comparison_bar>
  <figure>
    <img class=thumbnail alt="left image" src=${digestImagePath(ele.left.digest)}>
    <figcaption>
      <span class=legend_dot></span>
      <a href=${ele.left.detail}>${ele.left.title}</a>
    </figcaption>
  </figure>
  <a target=_blank rel=noopener href=${digestImagePath(ele.left.digest)}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
  ${comparison(ele)}
</div>
<button class=zoom_btn ?hidden=${!ele.right} @click=${ele._handleZoomClicked}>Zoom</button>
`;

const comparison = (ele) => {
  if (!ele.right) {
    return html`<div class=no_compare>No other images to compare against.</div>`;
  }
  const diffSrc = digestDiffImagePath(ele.left.digest, ele.right.digest);
  return html`
<img class=thumbnail alt="diff between left and right image" src=${diffSrc}>
<a target=_blank rel=noopener href=${diffSrc}>
  <open-in-new-icon-sk></open-in-new-icon-sk>
</a>

<figure>
  <img class=thumbnail alt="right image" src=${digestImagePath(ele.right.digest)}>
  <figcaption>
    <a href=${ele.right.detail}>${ele.right.title}</a>
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
      // We can't derive the detail url w/o also passing down issue (and maybe other things), so
      // for simplicity, we have the client compute what detail page they want to link to.
      detail: '',
    };
    this._right = null;
  }

  connectedCallback() {
    super.connectedCallback();
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

  _handleZoomClicked() {
    this.dispatchEvent(new CustomEvent('zoom-clicked', {
      bubbles: true,
      detail: {
        leftImgUrl: digestImagePath(this.left.digest),
        rightImgUrl: digestImagePath(this.right.digest),
        middleImgUrl: digestDiffImagePath(this.left.digest, this.right.digest),
        llabel: this.left.title,
        rlabel: this.right.title,
      },
    }));
  }
});
