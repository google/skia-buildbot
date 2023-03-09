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
 * @module elements-sk/toast-sk
 * @description <h2><code>toast-sk</code></h2>
 *
 * <p>
 *   Notification toast that pops up from the bottom of the screen when shown.
 *   Note that toast-sk doesn't add anyting inside the toast, it just presents
 *   the existing child elements.
 * </p>
 *
 * @attr duration - The duration, in ms, to display the notification. Defaults
 *               to 5000. A value of 0 means to display forever.
 */
import { define } from '../define';
import { upgradeProperty } from '../upgradeProperty';

export class ToastSk extends HTMLElement {
  private _timer: number | null;

  constructor() {
    super();
    this._timer = null;
  }

  connectedCallback(): void {
    if (!this.hasAttribute('duration')) {
      this.duration = 5000;
    }
    upgradeProperty(this, 'duration');
  }

  /** Mirrors the duration attribute. */
  get duration(): number { return +(this.getAttribute('duration') || ''); }

  set duration(val: number) { this.setAttribute('duration', val.toString()); }

  /** Displays the contents of the toast. */
  show(): void {
    this.setAttribute('shown', '');
    if (this.duration > 0 && !this._timer) {
      this._timer = window.setTimeout(() => {
        this._timer = null;
        this.hide();
      }, this.duration);
    }
  }

  /** Hides the contents of the toast. */
  hide(): void {
    this.removeAttribute('shown');
    if (this._timer) {
      window.clearTimeout(this._timer);
      this._timer = null;
    }
  }
}

define('toast-sk', ToastSk);
