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
 * @module elements-sk/collapse-sk
 * @description <h2><code>collapse-sk</code></h2>
 *
 * <p>
 *   Is a collapsable element, upon collapse the element and its children
 *   are no longer displayed.
 * </p>
 *
 *  @attr closed - A boolean attribute that, if present, causes the element to
 *     collapse, i.e., transition to display: none.
 *
 */
import { define } from '../define';
import { upgradeProperty } from '../upgradeProperty';

export class CollapseSk extends HTMLElement {
  connectedCallback(): void {
    upgradeProperty(this, 'closed');
  }

  /** Mirrors the closed attribute. */
  get closed(): boolean { return this.hasAttribute('closed'); }

  set closed(val: boolean) {
    if (val) {
      this.setAttribute('closed', '');
    } else {
      this.removeAttribute('closed');
    }
  }
}

define('collapse-sk', CollapseSk);
