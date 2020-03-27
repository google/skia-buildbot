/**
 * @module module/image-compare-sk
 * @description <h2><code>image-compare-sk</code></h2>
 *
 * Shows a side by side comparison of two images. If there's nothing to compare against, it will
 * only display one.
 *
 * @evt zoom-clicked - When the zoom button is clicked, this event is produced. It has a detail
 *   object with the following string properties: leftImgUrl, rightImgUrl, middleImgUrl, llabel,
 *   rlabel. TODO(kjlubick) can we have this element have/show the dialog?
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/open-in-new-icon-sk';

const template = (ele) => html`
<div class=comparison_bar>
  <figure>
    <div class=preview>
      <img alt="left image" src=${ele.left.src}>
    </div>
    <figcaption><span class=legend_dot></span>
      <a href=${ele.left.detail}>${ele.left.title}</a>
    </figcaption>
  </figure>
  <a target=_blank rel=noopener href=${ele.left.src}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
  ${comparison(ele)}
</div>
<button class=zoom_btn ?hidden=${!ele.right} @click=${ele._handleZoomClicked}>ZOOM</button>
`;

const comparison = (ele) => {
  if (!ele.right) {
    return html`<div class=no_compare>No other images to compare against.</div>`;
  }
  return html`
<div class=preview>
  <img alt="diff between left and right image" src=${ele.diff.src}>
</div>
 <a target=_blank rel=noopener href=${ele.diff.src}>
  <open-in-new-icon-sk></open-in-new-icon-sk>
</a>

<figure>
  <div class=preview>
    <img alt="right image" src=${ele.right.src}>
  </div>
  <figcaption>
    <a href=${ele.right.detail}>${ele.right.title}</a>
  </figcaption>
</figure>
<a target=_blank rel=noopener href=${ele.right.src}>
  <open-in-new-icon-sk></open-in-new-icon-sk>
</a>`;
};

define('image-compare-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._left = {
      src: '',
      title: '',
      detail: '',
    };
    this._diff = null;
    this._right = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop diff {object} an object with string property src.
   */
  get diff() { return this._diff; }

  set diff(obj) {
    this._diff = obj;
    this._render();
  }

  /**
   * @prop left {object} an object with string properties src, title, detail.
   */
  get left() { return this._left; }

  set left(obj) {
    this._left = obj;
    this._render();
  }

  /**
   * @prop right {object} an object with string properties src, title, detail.
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
        leftImgUrl: this._left.src,
        rightImgUrl: this._right.src,
        middleImgUrl: this._diff.src,
        llabel: this._left.title,
        rlabel: this._right.title,
      },
    }));
  }
});
