/**
 * @module modules/multi-input-sk
 * @description <h2><code>multi-input-sk</code></h2>
 *
 * multi-input-sk behaves similarly to <input type="text"> but its value is a
 * string[].
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../ElementSk';
import 'elements-sk/icon/close-icon-sk';

export class MultiInputSk extends ElementSk {
  private static template = (ele: MultiInputSk) => html`
    <div class="input-container">
      ${ele._values.map(
    (value: string, index: number) => html`
          <div class="input-item">
            ${value}
            <a
              @click=${() => {
      ele._values.splice(index, 1);
      ele._render();
      ele.dispatchEvent(new Event('change', { bubbles: true }));
    }}
            >
              <close-icon-sk></close-icon-sk>
            </a>
          </div>
        `,
  )}
      <input type="text" @change=${(ev: Event) => {
    ev.stopPropagation();
    const inp = $$<HTMLInputElement>('input', ele)!;
    ele._values.push(inp.value);
    inp.value = '';
    ele._render();
    ele.dispatchEvent(new Event('change', { bubbles: true }));
  }}></input>
    </div>
  `;

  private _values: string[] = [];

  get values(): string[] {
    return this._values.slice();
  }

  set values(values: string[]) {
    this._values = values.slice();
    this._render();
  }

  constructor() {
    super(MultiInputSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('multi-input-sk', MultiInputSk);
