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

  private static template = (ele: TestSrcSk) =>
    html`<pre class="output">${ele._text}</pre>`;

  constructor() {
    super(TestSrcSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('src');
  }

  /** @prop src - URL to retrieve the text from. */
  get src() {
    return this._src;
  }
  set src(val) {
    this._src = val;
    // Property funcs can't be async, thus the IIAAFE (immediately invoked async
    // arrow function expression).
    (async () => {
      if (val === '') {
        return;
      }
      const resp = await fetch(val);
      if (!resp.ok) {
        throw `Failed to retrieve text from ${val}`;
      }
      this._text = await resp.text();
      this._render();
      this.dispatchEvent(new CustomEvent('change', { bubbles: true }));
    })();
  }
}

define('test-src-sk', TestSrcSk);
