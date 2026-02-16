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
 * @module elements-sk/tabs-panel-sk
 * @description <h2><code>tabs-panel-sk</code></h2>
 *
 * <p>
 *   See the description of [tabs-sk]{@link module:elements-sk/tabs-sk}.
 * </p>
 *
 * @attr selected - The index of the tab panel to display.
 *
 */
import { define } from '../define';
import { upgradeProperty } from '../upgradeProperty';

export class TabsPanelSk extends HTMLElement {
  static get observedAttributes(): string[] {
    return ['selected'];
  }

  connectedCallback(): void {
    upgradeProperty(this, 'selected');
  }

  /** Mirrors the 'selected' attribute. */
  get selected(): number {
    return +(this.getAttribute('selected') || '');
  }

  set selected(val: number) {
    this.setAttribute('selected', String(val));
    this._select(val);
  }

  attributeChangedCallback(_name: string, _oldValue: any, newValue: any): void {
    this._select(+newValue);
  }

  private _select(index: number): void {
    for (let i = 0; i < this.children.length; i++) {
      this.children[i].classList.toggle('selected', i === index);
    }
  }
}

define('tabs-panel-sk', TabsPanelSk);
