/**
 * @module modules/matrix-clip-controls-sk
 * @description <h2><code>matrix-clip-controls-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/checkbox-sk';

export interface MatrixClipInfo {
  matrixSize: number; // 3 or 4
  clip: number[]; // array of left top right bottom
  matrix: number[][]; // 3x3 or 4x4 depending on matrixSize
}

export class MatrixClipControlsSk extends ElementSk {
  private static template = (ele: MatrixClipControlsSk) =>
    html`<div>
      Bounds and Matrix
      <checkbox-sk label="Show Clip"
                   title="Show a semi-transparent teal overlay on the areas within the current clip."
                   id=clip class="clipcheckbox" on-change="_clipHandler"></checkbox-sk>
      <checkbox-sk label="Show Android Device Clip Restriction"
                   title="Show a semi-transparent peach overlay on the areas within the current andorid device clip restriction. This is set at the beginning of each frame and recorded in the DrawAnnotation Command labeled AndroidDeviceClipRestriction"
                   id=androidclip class="clipcheckbox" on-change="_androidClipHandler"></checkbox-sk>
      <checkbox-sk label="Show Origin"
                   title="Show the origin of the coordinate space defined by the current matrix."
                   id=origin class="clipcheckbox" on-change="_originHandler"></checkbox-sk>
      <h2>Clip</h2>
      <table>
        <tr><td>${ele.info.clip.0}</td><td>${ ele.info.clip.1 }</td></tr>
        <tr><td>${ele.info.clip.2}</td><td>${ ele.info.clip.3 }</td></tr>
      </table>
      <h2>Matrix</h2>
      <table>
        <tr><td>${ ele.info.matrix.0.0 }</td><td>${ ele.info.matrix.0.1 }</td><td>${ ele.info.matrix.0.2 }</td></tr>
        <tr><td>${ ele.info.matrix.1.0 }</td><td>${ ele.info.matrix.1.1 }</td><td>${ ele.info.matrix.1.2 }</td></tr>
        <tr><td>${ ele.info.matrix.2.0 }</td><td>${ ele.info.matrix.2.1 }</td><td>${ ele.info.matrix.2.2 }</td></tr>
      </table>
    </div>`;


  public info: MatrixClipInfo = {
    matrixSize: 3,
    clip: 0,2,100,200,
    matrix: [
      [0, 1, 2],
      [3, 4, 5],
      [6, 7, 8],
    ],
  };

  constructor() {
    super(MatrixClipControlsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('matrix-clip-controls-sk', MatrixClipControlsSk);
