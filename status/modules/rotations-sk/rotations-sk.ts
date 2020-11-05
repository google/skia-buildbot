/**
 * @module modules/rotations-sk
 * @description <h2><code>rotations-sk</code></h2>
 *
 * Custom element for displaying current rotations for Skia, GPU, Android, and Infra.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { unsafeHTML } from 'lit-html/directives/unsafe-html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Rotation } from '../tree-status-sk/tree-status-sk';

export class RotationsSk extends ElementSk {
  private _rotations: Array<Rotation> = [];
  private static template = (el: RotationsSk) => html`
    <div class="table">
      ${el._rotations.map(
        (rotation) => html`
          <a
            class="tr"
            href=${rotation.docLink}
            target="_blank"
            rel="noopener noreferrer"
          >
            <div class="td">
              ${unsafeHTML(
                `<${rotation.icon}-icon-sk></${rotation.icon}-icon-sk>`
              )}
              ${rotation.role}: ${rotation.name}
            </div>
          </a>
        `
      )}
    </div>
  `;

  constructor() {
    super(RotationsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  set rotations(value: Array<Rotation>) {
    this._rotations = value;
    this._render();
  }
}

define('rotations-sk', RotationsSk);
