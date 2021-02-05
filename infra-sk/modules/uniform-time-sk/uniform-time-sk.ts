/**
 * @module modules/uniform-time-sk
 * @description <h2><code>uniform-time-sk</code></h2>
 *
 * Constructs a handler for the iTime uniform.
 *
 * Displays the play/pause and rewind buttons.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/fast-rewind-icon-sk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iTime',
  rows: 1,
  columns: 1,
  slot: 0,
};

type DateNow = () => number;

export class UniformTimeSk extends ElementSk implements UniformControl {
  private startTime: number = 0;

  private pauseTime: number = 0;

  private _uniform: Uniform = defaultUniform;

  private _dateNow: DateNow = Date.now;

  private playing: boolean = true;

  private static template = (ele: UniformTimeSk) => html`
    <button @click=${ele.restart}>
      <fast-rewind-icon-sk></fast-rewind-icon-sk>
    </button>
    <button @click=${ele.togglePlaying}>
      <play-arrow-icon-sk ?hidden=${ele.playing}></play-arrow-icon-sk>
      <pause-icon-sk ?hidden=${!ele.playing}></pause-icon-sk>
    </button>
    <span>${ele.time}</span>
    <span>${ele._uniform.name}</span>
  `;

  constructor() {
    super(UniformTimeSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.startTime = this.dateNow();
  }

  private restart() {
    this.startTime = this.dateNow();
    this.pauseTime = 0;
    this._render();
  }

  private togglePlaying() {
    if (this.playing) {
      this.pauseTime = this.time;
    } else {
      this.startTime = this.dateNow() - this.time * 1000;
    }
    this.playing = !this.playing;
    this._render();
  }

  render(): void {
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: Float32Array): void {
    uniforms[this._uniform.slot] = this.time;
  }

  /** Allows overriding the Date.now function for testing. */
  get dateNow() {
    return this._dateNow;
  }
  set dateNow(val) {
    this._dateNow = val;
  }

  /** The current time offset in seconds. */
  get time() {
    if (!this.playing) {
      return this.pauseTime;
    }
    return (this.dateNow() - this.startTime) / 1000;
  }

  /** The description of the uniform. */
  get uniform(): Uniform {
    return this._uniform;
  }

  set uniform(val: Uniform) {
    if (val.rows !== 1 || val.columns !== 1) {
      throw new Error('Invalid time uniform dimensions.');
    }
    this._uniform = val;
    this._render();
  }
}

define('uniform-time-sk', UniformTimeSk);
