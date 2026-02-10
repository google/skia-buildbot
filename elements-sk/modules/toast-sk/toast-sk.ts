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
import { noChange, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('toast-sk')
export class ToastSk extends LitElement {
  @property({ type: Number, reflect: true })
  duration: number = 5000;

  @property({ type: Boolean, reflect: true })
  shown: boolean = false;

  private _timer: number | null = null;

  createRenderRoot() {
    return this;
  }

  render() {
    // We use Light DOM and want to preserve existing children (like error-toast-sk's content),
    // so we return noChange to prevent Lit from modifying the child list.
    return noChange;
  }

  /** Displays the contents of the toast. */
  show(): void {
    this.shown = true;
    if (this.duration > 0 && !this._timer) {
      this._timer = window.setTimeout(() => {
        this._timer = null;
        this.hide();
      }, this.duration);
    }
  }

  /** Hides the contents of the toast. */
  hide(): void {
    this.shown = false;
    if (this._timer) {
      window.clearTimeout(this._timer);
      this._timer = null;
    }
  }
}
