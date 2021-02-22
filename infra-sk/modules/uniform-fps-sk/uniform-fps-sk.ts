/**
 * @module modules/uniform-fps-sk
 * @description <h2><code>uniform-fps-sk</code></h2>
 *
 * Displays the frames per second.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { FPS } from '../fps/fps';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'raf',
  rows: 0,
  columns: 0,
  slot: 0,
};

export class UniformFpsSk extends ElementSk implements UniformControl {
    uniform: Uniform = defaultUniform;

    private fps: FPS = new FPS();

    constructor() {
      super(UniformFpsSk.template);
    }

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    private static template = (ele: UniformFpsSk) => html`fps`;

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    applyUniformValues(uniforms: number[]): void {
      // noop as UniformRafSk doesn't supply uniforms.
    }

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    restoreUniformValues(uniforms: number[]): void {
      // noop as UniformRafSk doesn't supply uniforms.
    }

    onRAF(): void {
      this.fps.raf();
      this.textContent = `${this.fps.fps.toFixed(0)} fps`;
    }

    needsRAF(): boolean {
      return true;
    }

    connectedCallback(): void {
      super.connectedCallback();
      this._render();
    }
}

define('uniform-fps-sk', UniformFpsSk);
