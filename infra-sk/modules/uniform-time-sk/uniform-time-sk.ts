/**
 * @module modules/uniform-time-sk
 * @description <h2><code>uniform-time-sk</code></h2>
 *
 * Constructs a handler for the iTime uniform.
 *
 * Displays the play/pause and rewind buttons.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../ElementSk';
import '../../../elements-sk/modules/icons/play-arrow-icon-sk';
import '../../../elements-sk/modules/icons/pause-icon-sk';
import '../../../elements-sk/modules/icons/fast-rewind-icon-sk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iTime',
  rows: 1,
  columns: 1,
  slot: 0,
};

// The type of Date.now.
type DateNow = () => number;

export class UniformTimeSk extends ElementSk implements UniformControl {
  private startTime: number = 0; // The time as recorded from this._dateNow in ms.

  private pauseTime: number = 0; // The time we were at when pause was pressed, in seconds.

  private _uniform: Uniform = defaultUniform;

  private _dateNow: DateNow = Date.now; // A replaceable Date.now() function for testing.

  private playing: boolean = true;

  constructor() {
    super(UniformTimeSk.template);
  }

  private static template = (ele: UniformTimeSk) => html`
    <button id="restart" @click=${ele.restart}>
      <fast-rewind-icon-sk></fast-rewind-icon-sk>
    </button>
    <button id="playpause" @click=${ele.togglePlaying}>
      <play-arrow-icon-sk ?hidden=${ele.playing}></play-arrow-icon-sk>
      <pause-icon-sk ?hidden=${!ele.playing}></pause-icon-sk>
    </button>
    <span id="ms">${ele.time.toFixed(3)}</span>
    <span>${ele._uniform.name}</span>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.startTime = this.dateNow();
    this.time = 0;
  }

  render(): void {
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: number[]): void {
    uniforms[this._uniform.slot] = this.time;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  restoreUniformValues(uniforms: number[]): void {
    // This is a noop, we don't restore predefined uniform values.
  }

  onRAF(): void {
    this.render();
  }

  needsRAF(): boolean {
    return true;
  }

  /** Allows overriding the Date.now function for testing. */
  get dateNow(): DateNow {
    return this._dateNow;
  }

  set dateNow(val: DateNow) {
    this._dateNow = val;
  }

  /** The current time offset in seconds. */
  get time(): number {
    if (!this.playing) {
      return this.pauseTime;
    }
    return (this.dateNow() - this.startTime) / 1000;
  }

  set time(val: number) {
    this.pauseTime = val;
    this.startTime = this.dateNow() - val * 1000;
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

  private restart() {
    this.time = 0;
    this.pauseTime = 0;
    this._render();
  }

  private togglePlaying() {
    if (this.playing) {
      this.pauseTime = this.time;
    } else {
      this.time = this.pauseTime;
    }
    this.playing = !this.playing;
    this._render();
  }
}

define('uniform-time-sk', UniformTimeSk);
