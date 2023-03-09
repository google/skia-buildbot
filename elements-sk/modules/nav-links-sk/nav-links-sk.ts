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
 * @module elements-sk/nav-links-sk
 * @description <h2><code>nav-links-sk</code></h2>
 *
 * <p>
 *   See also the documentation for {@link module:elements-sk/nav-button-sk}.
 * </p>
 *
 * <p>
 *   The nav-links-sk will be closed if the user presses ESC, or if focus
 *   moves off of nav-links-sk or any of its children.
 * </p>
 *
 * @attr shown - A boolean attribute controlling if the list of links is
 *   displayed or not.
 *
 * @example
 *
 * // Using nav-button-sk:
 * <nav-button-sk></nav-button-sk>
 * <nav-links-sk shown>
 *   <a href="">Main</a>
 *   <a href="">Triage</a>
 *   <a href="">Alerts</a>
 * </nav-links-sk>
 *
 * // Using a button instead of nav-button-sk:
 * <button onclick="this.nextElementSibling.shown = true;">
 *   <create-icon-sk></create-icon-sk>
 * </button>
 * <nav-links-sk>
 *   <a href="">New A</a>
 *   <a href="">New B</a>
 *   <a href="">New C</a>
 * </nav-links-sk>
 *
 */
import { define } from '../define';
import { upgradeProperty } from '../upgradeProperty';

export class NavLinksSk extends HTMLElement {
  static get observedAttributes(): string[] {
    return ['shown'];
  }

  connectedCallback(): void {
    upgradeProperty(this, 'shown');
  }

  /** Mirrors the shown attribute. */
  get shown(): boolean { return this.hasAttribute('shown'); }

  set shown(val: boolean) {
    if (val) {
      this.setAttribute('shown', '');
    } else {
      this.removeAttribute('shown');
    }
  }

  attributeChangedCallback(name: string, oldValue: any, newValue: any): void {
    if (newValue !== null) {
      window.addEventListener('keydown', this);
    } else {
      window.removeEventListener('keydown', this);
      this.dispatchEvent(new CustomEvent('closed', { bubbles: true }));
    }
  }

  handleEvent(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      this.shown = false;
    }
  }
}

define('nav-links-sk', NavLinksSk);
