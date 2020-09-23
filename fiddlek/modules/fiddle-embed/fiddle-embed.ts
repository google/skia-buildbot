/**
 * @module modules/fiddle-embed
 * @description <h2><code>fiddle-embed</code></h2>
 *
 * A control for embedding a fiddle as a custom element on a different domain.
 *
 * @attr name - The name/fiddlehash of the fiddle.
 * @attr gpu  - If present then use the GPU image/webm output instead of the CPU
 *      output.
 * @attr cpu  - Force showing the cpu image output even if 'gpu' is true. I.e.
 *      both cpu and gpu will be displayed.
 *
 * @event context-loaded is sent when the context for a fiddle had been loaded from the server.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../fiddle-sk';
import { FiddleContext } from '../json';
import { Config, FiddleSk } from '../fiddle-sk/fiddle-sk';

export class FiddleEmbed extends ElementSk {
  /** The default configuration for the <fiddle-sk> element. */
  private config: Config = {
    display_options: false,
    embedded: true,
    cpu_embedded: false,
    gpu_embedded: false,
    options_open: false,
    basic_mode: true,
    domain: 'https://fiddle.skia.org',
    bug_link: false,
    sources: [1, 2, 3, 4, 5, 6],
    loop: true,
    play: true,
  }

  private context: FiddleContext | null = null;

  private control: FiddleSk | null = null;

  constructor() {
    super(FiddleEmbed.template);
    this._render();
  }

  private static template = (ele: FiddleEmbed) => html`<fiddle-sk
    .config=${ele.mergedConfig()}
    .context=${ele.context}></fiddle-sk>
    <error-toast-sk></error-toast-sk>
    `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.control = this.querySelector('fiddle-sk');
  }

  attributeChangedCallback(name: string, oldValue: string, newValue: string): void {
    if (name === 'name') {
      if (!newValue) {
        return;
      }
      fetch(`${this.config.domain}/e/${newValue}`).then(jsonOrThrow).then((json) => {
        this.control!.context = json;
        this._render();
        this.dispatchEvent(new CustomEvent('context-loaded', { bubbles: true }));
      }).catch(errorMessage);
    } else {
      this._render();
    }
  }

  /** Returns the default config with overrides from the cpu and gpu attributes. */
  private mergedConfig(): Config {
    return Object.assign({}, this.config, {
      cpu_embedded: this.hasAttribute('cpu'),
      gpu_embedded: this.hasAttribute('gpu'),
    });
  }

  static get observedAttributes(): string[] {
    return ['name', 'gpu', 'cpu'];
  }
}

define('fiddle-embed', FiddleEmbed);
