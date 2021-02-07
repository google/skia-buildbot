/**
 * @module modules/uniform-mouse-sk
 * @description <h2><code>uniform-mouse-sk</code></h2>
 *
 * Control to handle mouse position and clicks as a uniform.
 *
 * Note this control doesn't display anything.
 *
 * See https://www.shadertoy.com/view/Mss3zH for an explanation of how the
 * iMouse uniform behaves.
 */
import { define } from 'elements-sk/define';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iMouse',
  rows: 4,
  columns: 1,
  slot: 0,
};

export class UniformMouseSk extends HTMLElement implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  private _elementToMonitor: HTMLElement | null = null;

  private x: number= 0;

  private y: number= 0;

  private clickX: number = 1;

  private clickY: number = 1;

  private mouseDown: boolean = false;

  private mouseClick: boolean = false;

  applyUniformValues(uniforms: Float32Array): void {
    uniforms[this._uniform.slot] = this.x;
    uniforms[this._uniform.slot + 1] = this.y;
    uniforms[this._uniform.slot + 2] = Math.abs(this.clickX) * (this.mouseDown ? 1 : -1);
    uniforms[this._uniform.slot + 3] = Math.abs(this.clickY) * (this.mouseClick ? 1 : -1);
  }

  get elementToMonitor(): HTMLElement {
    return this._elementToMonitor!;
  }

  set elementToMonitor(val: HTMLElement) {
    if (this.elementToMonitor === val) {
      return;
    }
    if (this.elementToMonitor) {
      this._elementToMonitor!.removeEventListener('mouseup', this.mouseUpHandler.bind(this));
      this._elementToMonitor!.removeEventListener('mousedown', this.mouseDownHandler.bind(this));
      this._elementToMonitor!.removeEventListener('mousemove', this.mouseMoveHandler.bind(this));
      this._elementToMonitor!.removeEventListener('click', this.clickHandler.bind(this));
    }
    this._elementToMonitor = val;
    this._elementToMonitor!.addEventListener('mouseup', this.mouseUpHandler.bind(this));
    this._elementToMonitor!.addEventListener('mousedown', this.mouseDownHandler.bind(this));
    this._elementToMonitor!.addEventListener('mousemove', this.mouseMoveHandler.bind(this));
    this._elementToMonitor!.addEventListener('click', this.clickHandler.bind(this));
  }

  private mouseUpHandler(e: MouseEvent) {
    this.mouseDown = false;
    this.x = e.offsetX;
    this.y = e.offsetY;
  }

  private mouseDownHandler(e: MouseEvent) {
    this.mouseDown = true;
    this.x = e.offsetX;
    this.y = e.offsetY;
  }


  private mouseMoveHandler(e: MouseEvent) {
    this.x = e.offsetX;
    this.y = e.offsetY;
  }

  private clickHandler(e: MouseEvent) {
    this.clickX = e.offsetX;
    this.clickY = e.offsetY;
    this.mouseClick = true;
  }

  get uniform(): Uniform {
    return this._uniform;
  }

  set uniform(val: Uniform) {
    if (val.columns !== 4 || val.rows !== 1) {
      throw new Error('The mouse uniform must be a float4.');
    }
    this._uniform = val;
  }
}

define('uniform-mouse-sk', UniformMouseSk);
