/**
 * @module module/image-compare-sk
 * @description <h2><code>image-compare-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/open-in-new-icon-sk';

const template = (ele) => html`
<div class="flex_horizontal">

  <figure class="flex_vertical">
    <div class=preview>
      <img alt="left image" src=${ele._leftImageHref}>
    </div>
    <figcaption>${ele._leftTitle}</figcaption>
  </figure>
  <a target=_blank rel=noopener href=${ele._leftImageHref}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
  
  <div class=preview>
    <img alt="diff between left and right image" src=${ele._diffImageHref}>
  </div>
   <a target=_blank rel=noopener href=${ele._diffImageHref}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
  
  <figure class="flex_vertical">
    <div class=preview>
      <img alt="right image" src=${ele._rightImageHref}>
    </div>
    <figcaption>${ele._rightTitle}</figcaption>
  </figure>
  <a target=_blank rel=noopener href=${ele._rightImageHref}>
    <open-in-new-icon-sk></open-in-new-icon-sk>
  </a>
</div>
`;

define('image-compare-sk', class extends ElementSk {
  constructor() {
    super(template);

    // FIXME(kjlubick)
    this._leftImageHref = 'https://skia-infra-gold.skia.org/img/images/6246b773851984c726cb2e1cb13510c2.png';
    this._diffImageHref = 'https://skia-infra-gold.skia.org/img/diffs/6246b773851984c726cb2e1cb13510c2-99c58c7002073346ff55f446d47d6311.png';
    this._rightImageHref = 'https://skia-infra-gold.skia.org/img/images/99c58c7002073346ff55f446d47d6311.png';
    this._leftTitle = '6246b7738519...';
    this._rightTitle = 'Closest Negative';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
