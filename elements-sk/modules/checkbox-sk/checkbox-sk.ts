/* eslint-disable no-self-assign */
// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * @module elements-sk/checkbox-sk
 * @description <h2><code>checkbox-sk</code></h2>
 *
 * <p>
 *   The checkbox-sk and element contains a native 'input'
 *   element in light DOM so that it can participate in a form element.
 * </p>
 *
 * <p>
 *    Each element also supports the following attributes exactly as the
 *    native checkbox element:
 *    <ul>
 *      <li>checked</li>
 *      <li>disabled</li>
 *      <li>name</li>
 *     </ul>
 * </p>
 *
 * <p>
 *    All the normal events of a native checkbox are supported.
 * </p>
 *
 * @attr label - A string, with no markup, that is to be used as the label for
 *            the checkbox. If you wish to have a label with markup then set
 *            'label' to the empty string and create your own
 *            <code>label</code> element in the DOM with the 'for' attribute
 *            set to match the name of the checkbox-sk.
 *
 * @prop checked This mirrors the checked attribute.
 * @prop disabled This mirrors the disabled attribute.
 * @prop name This mirrors the name attribute.
 * @prop label This mirrors the label attribute.
 *
 */
import { define } from '../define';
import { upgradeProperty } from '../upgradeProperty';

export class CheckOrRadio extends HTMLElement {
  private static nextUniqueId = 0;

  protected readonly uniqueId = `${CheckOrRadio.nextUniqueId++}`;

  protected get _role() {
    return 'checkbox';
  }

  static get observedAttributes() {
    return ['checked', 'disabled', 'name', 'label'];
  }

  private _label: HTMLSpanElement | null = null;

  private _input: HTMLInputElement | null = null;

  content: string = `<label for="${this._role}-${this.uniqueId}">
    <input type=${this._role} id="${this._role}-${this.uniqueId}"></input>
    <span class=icons>
    <span class="icon-sk unchecked">check_box_outline_blank</span>
    <span class="icon-sk checked">check_box</span>
  </span>
  <span class=label></span></label>`;

  connectedCallback() {
    this.innerHTML = this.content;

    this._label = this.querySelector<HTMLSpanElement>('.label');
    this._input = this.querySelector<HTMLInputElement>('input');
    upgradeProperty(this, 'checked');
    upgradeProperty(this, 'disabled');
    upgradeProperty(this, 'name');
    upgradeProperty(this, 'label');
    // Since attributeChangedCallback can be called before connectedCallback,
    // and we have non-trivial actions when the attributes change to reflect
    // the values to this._input, we need trigger each setter on
    // connectedCallback.
    this.checked = this.checked;
    this.disabled = this.disabled;
    this.name = this.name;
    this.label = this.label;

    // TODO(jcgregorio) Do we capture and alter the 'input' and 'change' events generated
    // by the input element so that the evt.target points to 'this'?
    this._input!.addEventListener('change', (e) => {
      this.checked = (e.target as HTMLInputElement).checked;
    });
    this.addEventListener('click', (e) => {
      if (e.target === this) {
        if (this.checked && this._role === 'radio') {
          return;
        }
        this._input!.click();
      }
    });
  }

  get checked(): boolean {
    return this.hasAttribute('checked');
  }

  set checked(val: boolean) {
    // The attribute is the source of truth, not the property.
    if (val) {
      this.setAttribute('checked', '');
    } else {
      this.removeAttribute('checked');
    }
  }

  get disabled(): boolean {
    return this.hasAttribute('disabled');
  }

  set disabled(val: boolean) {
    if (val) {
      this.setAttribute('disabled', '');
    } else {
      this.removeAttribute('disabled');
    }
  }

  get name(): string {
    return this.getAttribute('name') || '';
  }

  set name(val: string) {
    if (val === null || val === undefined) {
      return;
    }
    this.setAttribute('name', val);
  }

  get label(): string {
    return this.getAttribute('label') || '';
  }

  set label(val: string) {
    if (val === null || val === undefined) {
      return;
    }
    this.setAttribute('label', val);
  }

  attributeChangedCallback(name: string, _oldValue: string | null, newValue: string | null): void {
    if (!this._input) {
      return;
    }
    // Strictly check for null since an empty string doesn't mean false
    // for a boolean attribute.
    const isTrue = newValue !== null;
    switch (name) {
      case 'checked':
        this._input.checked = isTrue;
        break;
      case 'disabled':
        this._input.disabled = isTrue;
        break;
      case 'name':
        this._input.name = newValue || '';
        break;
      case 'label':
        if (this._label) {
          this._label.textContent = newValue;
        }
        break;
      default:
        break;
    }
  }
}

define('checkbox-sk', CheckOrRadio);

export const sayHello = () => 'hello';
