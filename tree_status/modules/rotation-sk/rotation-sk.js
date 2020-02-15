/**
 * @module rotation-sk
 * @description <h2><code>rotation-sk</code></h2>
 *
 *   Displays the recent tree statuses.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import 'elements-sk/error-toast-sk'

const template = (ele) => html`
<div class="title">${ele.rotation_type} Rotation Schedule</div>
<br/>
<img border="0" src="/static/${ele._rotation_img}" width="180" height="120">
<br/>
<div class="note">
  ${ele._rotation_type}s : Please contact rmistry@ or skiabot@ if you need to swap your schedule.
  <br/>
  Documentation for ${ele._rotation_type}s is <a href='${ele._rotation_doc}'>here</a>.
</div>
<br/> 
<table class="rotations">
  <tr>
    <th>Who</th>
    <th>When</th>
  </tr>
</table>
`;

// ${getRotationRows(ele._rotations)}

function getRotationRows(rotations) {
  return rotations.map(rotation => html`
<tr>
  <td>${rotation.username}</td>
  <td>${rotation.readable_range}</td>
</tr>
`);
}

define('rotation-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._rotation_type = '';
    this._rotation_doc = '';
    this._rotations = [];
    this._rotation_img = '';
    console.log("CONSTRUCTOR!!!!!!!!!1");
  }

  /** @prop rotationType {string} The type of rotation (Sheriff/Trooper/etc). */
  get rotationType() { console.log("GETTING rotation Type"); return this._rotation_type; }
  set rotationType(val) {
    console.log("SETTING THIS:");
    console.log(val);
    this._rotation_type = val;
    this._render();
  }

  /** @prop rotationDoc {string} The doc for this rotation. */
  get rotationDoc() { return this._rotation_doc; }
  set rotationDoc(val) {
    console.log("SETTING THIS:");
    this._rotation_doc = val;
    this._render();
  }

  /** @prop rotations {Array of Objects} List of Rotations. */
  get rotations() { return this._rotations; }
  set rotations(val) {
    console.log("SETTING THIS:");
    this._rotations = val;
    this._render();
  }

  /** @prop rotationImg {string} Img to display for the rotation. */
  get rotationImg() { return this._rotation_img }
  set rotationImg(val) {
    console.log("SETTING THIS:");
    this._rotation_img = val;
    this._render();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    console.log("IN CONNECTED CALLBACK!!");
  }

});
