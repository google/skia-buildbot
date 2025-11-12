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
 * @module elements-sk/radio-sk
 * @description <h2><code>radio-sk</code></h2>
 *
 * <p>
 *   The radio-sk and element contains a native 'input'
 *   element in light DOM so that it can participate in a form element.
 * </p>
 *
 * <p>
 *    Each element also supports the following attributes exactly as the
 *    native radio button element:
 *    <ul>
 *      <li>checked</li>
 *      <li>disabled</li>
 *      <li>name</li>
 *     </ul>
 * </p>
 *
 * <p>
 *    All the normal events of a native radio button are supported.
 * </p>
 *
 * @attr label - A string, with no markup, that is to be used as the label for
 *            the radio button. If you wish to have a label with markup then set
 *            'label' to the empty string and create your own
 *            <code>label</code> element in the DOM with the 'for' attribute
 *            set to match the name of the radio-sk.
 *
 * @prop checked This mirrors the checked attribute.
 * @prop disabled This mirrors the disabled attribute.
 * @prop name This mirrors the name attribute.
 * @prop label This mirrors the label attribute.
 */

// Note that we are importing just the class, not the whole element + CSS definition.
import { define } from '../define';
import { CheckOrRadio } from '../checkbox-sk/checkbox-sk';

export class RadioElement extends CheckOrRadio {
  constructor() {
    super();
    this.content = `<label for="${this._role}-${this.uniqueId}">
      <input type=${this._role} id="${this._role}-${this.uniqueId}"></input>
      <span class=icons>
      <span class="icon-sk unchecked">radio_button_unchecked</span>
      <span class="icon-sk checked">radio_button_checked</span>
    </span>
    <span class=label></span></label>`;
  }

  protected get _role(): string {
    return 'radio';
  }
}

// The radio-sk element contains a native 'input' element in light DOM
// so that the radio button can participate in a form element,
// and also participate in a native 'radiogroup' element.
//
// See documentation for checkbox-sk.
define('radio-sk', RadioElement);
