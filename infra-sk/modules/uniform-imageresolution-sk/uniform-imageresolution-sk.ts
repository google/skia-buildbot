/**
 * @module modules/uniform-imageresolution-sk
 * @description <h2><code>uniform-imageresolution-sk</code></h2>
 *
 * Control to handle the input image resolution.
 *
 * Note this control doesn't display anything.
 */
import { define } from 'elements-sk/define';
import { Uniform, UniformControl } from '../uniform/uniform';

// All source images are 512 x 512 px.
export const imageSize = 512;

const defaultUniform: Uniform = {
  name: 'iImageResolution',
  rows: 3,
  columns: 1,
  slot: 0,
};

export class UniformImageresolutionSk extends HTMLElement implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  applyUniformValues(uniforms: number[]): void {
    uniforms[this._uniform.slot] = imageSize;
    uniforms[this._uniform.slot + 1] = imageSize;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  restoreUniformValues(uniforms: number[]): void {
    // This is a noop, we don't restore predefined uniform values.
  }

  onRAF(): void {
    // noop
  }

  needsRAF(): boolean {
    return false;
  }

  get uniform(): Uniform {
    return this._uniform;
  }

  set uniform(val: Uniform) {
    if (val.columns !== 3 || val.rows !== 1) {
      throw new Error('The imageresolution uniform must be a float3.');
    }
    this._uniform = val;
  }
}

define('uniform-imageresolution-sk', UniformImageresolutionSk);
