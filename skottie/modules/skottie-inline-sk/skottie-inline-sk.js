/**
 * @module skottie/skottie-inline-sk
 * @description <h2><code>skottie-inline-sk</code></h2>
 *
 * Displays just the WASM based animation suitable for using inline in
 * documentation.
 *
 * @attr width The width of the animation. Over-rides with width
 *    of an animation stored at skottie.skia.org.
 *
 * @attr height The height of the animation. Over-rides with height
 *    of an animation stored at skottie.skia.org.
 *
 * @attr src The URL to load the animation from. The contents can be
 *    either a stored skottie, or a raw BodyMovin JSON file. The
 *    stored Skotties contain width and height, but those values
 *    can be overridden by specifying the width and height on this element.
 *
 * @example
 *
 *  <skottie-inline-sk width=128 height=128
 *    src="https://skottie.skia.org/e/1112d01d28a776d777cebcd0632da15b">
 *  </skottie-inline-sk>
 */
import '../skottie-player-sk'
import { define } from 'elements-sk/define'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

define('skottie-inline-sk', class extends HTMLElement {
  static get observedAttributes() {
    return ['width', 'height', 'src'];
  }

  constructor() {
    super();
    this._fetching = false;
  }

  connectedCallback() {
    this.innerHTML = `<skottie-player-sk></skottie-player-sk>`;
  }

  /** @prop width {string} Reflects the 'width' attribute.  */
  get width() { return this.getAttribute('width'); }
  set width(val) { this.setAttribute('width', val); }

  /** @prop height {string} Reflects the 'height' attribute.  */
  get height() { return this.getAttribute('height'); }
  set height(val) { this.setAttribute('height', val); }

  /** @prop src {string} Reflects the 'src' attribute. */
  get src() { return this.getAttribute('src'); }
  set src(val) { this.setAttribute('src', val); }

  _load() {
    if (!this.src) {
      return
    }
    this._fetching = true;
    fetch(this.src).then(jsonOrThrow).then(json => {
      this._fetching = false;
      let init = {
        width : 128,
        height : 128,
        lottie : json,
      }
      // If this is a file from skottie.skia.org.
      if (json.lottie !== undefined) {
        init.width = json.width;
        init.height = json.height;
        init.lottie = json.lottie;
      }
      if (this.width) {
        init.width = +this.width;
      }
      if (this.height) {
        init.height = +this.height;
      }
      let player = this.querySelector('skottie-player-sk');
      player.initialize(init);
    }).catch((msg) => {
      this._fetching = false;
    });
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (!this._fetching) {
      this._load();
    }
  }
});
