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
import '../skottie-player-sk';
import { define } from 'elements-sk/define';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';

export class SkottieInlineSk extends ElementSk {
  private static template = () => html`
    <skottie-player-sk></skottie-player-sk>`;

  private fetching: boolean = false;

  constructor() {
    super(SkottieInlineSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  get width(): string | null { return this.getAttribute('width'); }

  set width(val: string | null) {
    if (val) {
      this.setAttribute('width', val);
    }
  }

  get height(): string | null { return this.getAttribute('height'); }

  set height(val: string | null) {
    if (val) {
      this.setAttribute('height', val);
    }
  }

  get src(): string | null { return this.getAttribute('src'); }

  set src(val: string | null) {
    if (val) {
      this.setAttribute('src', val);
    }
  }

  load(): void {
    if (!this.src) {
      return;
    }
    this.fetching = true;
    fetch(this.src).then(jsonOrThrow).then((json) => {
      this.fetching = false;
      const init = {
        width: 128,
        height: 128,
        lottie: json,
        fps: 0,
      };
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
      const player = this.querySelector<SkottiePlayerSk>('skottie-player-sk');
      return player!.initialize(init);
    }).catch((msg) => {
      console.error(msg);
      this.fetching = false;
    });
  }

  static get observedAttributes(): string[] {
    return ['width', 'height', 'src'];
  }

  attributeChangedCallback(): void {
    if (!this.fetching) {
      this.load();
    }
  }
}
define('skottie-inline-sk', SkottieInlineSk);
