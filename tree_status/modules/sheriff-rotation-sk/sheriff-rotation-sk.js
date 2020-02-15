/**
 * @module sheriff-rotation-sk
 * @description <h2><code>sheriff-rotation-sk</code></h2>
 *
 *   Displays the recent tree statuses.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../rotation-sk'

import 'elements-sk/error-toast-sk'

const template = (ele) => html`
<rotation-sk .rotation_type=${ele._rotation_type}></rotation-sk>
`;

define('sheriff-rotation-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._rotation_type = 'SHERIFF';
    this._rotation_doc = '';
    this._rotations = [];
    this._rotation_img = '';
    console.log("CONSTRUCTOR!!!!!!!!!1");
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

});
