/**
 * @module modules/matrix-clip-controls-sk
 *
 * @description All the controls the appear on the right pane of the debugger,
 * and the accompanying matrix and clip feedback.
 *
 * TODO(nifong): Add GPU switch, light-dark switch, op bounds, overdraw
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/checkbox-sk'; // Import the element used in in the template
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk'; // import the type
import { Matrix3x3, Matrix4x4, MatrixClipInfo } from '../debugger';

export class MatrixClipControlsSk extends ElementSk {
  private static template = (ele: MatrixClipControlsSk) =>
    html`<div>
      Overlay Options
      <checkbox-sk label="Show Clip"
                   title="Show a semi-transparent teal overlay on the areas within the current clip."
                   id=clip @change=${ele._clipHandler}></checkbox-sk>
      <checkbox-sk label="Show Android Device Clip Restriction"
                   title="Show a semi-transparent peach overlay on the areas within the current andorid device clip restriction. This is set at the beginning of each frame and recorded in the DrawAnnotation Command labeled AndroidDeviceClipRestriction"
                   id=androidclip @change=${ele._androidClipHandler}></checkbox-sk>
      <checkbox-sk label="Show Origin"
                   title="Show the origin of the coordinate space defined by the current matrix."
                   id=origin @change=${ele._originHandler}></checkbox-sk>
      <h3>Clip</h3>
      <table>
        <tr><td>${ ele._info.clip[0] }</td><td>${ ele._info.clip[1] }</td></tr>
        <tr><td>${ ele._info.clip[2] }</td><td>${ ele._info.clip[3] }</td></tr>
      </table>
      <h3>Matrix</h3>
      <table>
        ${ele._matrixTable(ele._info.matrix)}
      </table>
    </div>`;


  private _info: MatrixClipInfo = {
    // These initial values do not matter.
    clip: [0, 0, 0, 0],
    matrix: [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
    ],
  };

  private _showClip = false;
  private _showAndroidClip = false;
  private _showOrigin = false;

  constructor() {
    super(MatrixClipControlsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // properties
  get info() {
    return this._info;
  }

  set info(newValue: MatrixClipInfo) {
    this._info = newValue;
    this._render();
  }

  // Template helper rendering a number[][] in a table
  private _matrixTable(m: Matrix3x3 | Matrix4x4) {
    return (m as number[][]).map((row: number[]) => {
      return html`<tr>${ row.map((i: number) => html`<td>${i}</td>`) }</tr>`;
    });
  }

  // controls change handlers
  private _clipHandler(e: Event) {
    this._showClip = (e.target as CheckOrRadio).checked;
  }

  private _androidClipHandler(e: Event) {
    this._showAndroidClip = (e.target as CheckOrRadio).checked;
  }

  private _originHandler(e: Event) {
    this._showOrigin = (e.target as CheckOrRadio).checked;
  }
};

define('matrix-clip-controls-sk', MatrixClipControlsSk);