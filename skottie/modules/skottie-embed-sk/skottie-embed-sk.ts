/**
 * @module skottie/skottie-embed-sk
 * @description <h2><code>skottie-embed-sk</code></h2>
 *
 * Displays just the WASM based animation suitable for iframing.
 *
 * @evt
 *
 * @attr
 *
 * @example
 *
 *  <iframe width=128 height=128
 *    src="https://skottie.skia.org/e/5c1c5cc9aa4aabe4acc1f12a7bac60fb?w=128&h=128"
 *    scrolling=no>
 *  </iframe>
 */
import '../skottie-player-sk';
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';

export class SkottieEmbedSk extends ElementSk {
  private static template = () =>
    html` <skottie-player-sk></skottie-player-sk>`;

  private hash: string = '';

  private height: number = 128;

  private width: number = 128;

  constructor() {
    super(SkottieEmbedSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.reflectFromURL();
  }

  reflectFromURL(): void {
    // Check URL.
    const match = window.location.pathname.match(/\/e\/([a-zA-Z0-9]+)/);
    if (!match) {
      // Make this the hash of the lottie file you want to play on startup.
      this.hash = '5c1c5cc9aa4aabe4acc1f12a7bac60fb'; // gear.json
    } else {
      this.hash = match[1];
    }

    stateReflector(
      /* getState */ () => ({
        w: this.width,
        h: this.height,
      }),
      /* setState */ (newState: HintableObject) => {
        this.width = +newState.w;
        this.height = +newState.h;
      }
    );

    // Run this on the next micro-task to allow mocks to be set up if needed.
    setTimeout(() => {
      fetch(`/_/j/${this.hash}`, {
        credentials: 'include',
      })
        .then(jsonOrThrow)
        .then((json) => {
          const player =
            this.querySelector<SkottiePlayerSk>('skottie-player-sk');
          return player!.initialize({
            width: this.width,
            height: this.height,
            lottie: json.lottie,
            fps: 0,
          });
        })
        .catch((msg) => {
          console.error(msg);
        });
    });
  }
}

define('skottie-embed-sk', SkottieEmbedSk);
