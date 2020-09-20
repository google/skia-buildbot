/**
 * @module modules/test-src-sk
 * @description <h2><code>test-src-sk</code></h2>
 *
 *  Displays text loaded from a URL in the same way an image loads and displays
 *  from a URL.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class TestSrcSk extends ElementSk {
  // The retrieved text.
  private _text: string = '';

  // The URL to retrieve the text from.
  private _src: string = '';

  constructor() {
    super(TestSrcSk.template);
  }

  private static template = (ele: TestSrcSk) => html`<pre class="output">${ele._text}</pre>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('src');
  }

  /** @prop src - URL to retrieve the text from. */
  get src(): string {
    return this._src;
  }

  set src(val: string) {
    this._src = val;
    fetch(val).then((resp) => {
      if (!resp.ok) {
        throw new Error(`Failed to retrieve text from ${val}`);
      }
      return resp.text();
    }).then((text) => {
      this._text = text;
      this._render();
      this.dispatchEvent(new CustomEvent('change', { bubbles: true }));
    });
  }
}

define('test-src-sk', TestSrcSk);
