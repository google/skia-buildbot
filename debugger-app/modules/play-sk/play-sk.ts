/**
 * @module modules/play-sk
 * @description A playback controller for looping over a sequence.
 *
 * @evt mode-changed-manually - After the user clicks the play/pause button
 *          (but not if you set the mode property from code)
 *           detail: {mode: PlayMode} // 'play' or 'pause'
 * @evt moveto - Indicating that the playback module wants to move the application to a new item
 *           The application should handle the event by updating itself to show the new item, then
 *           call movedTo(item) when finished to indicate playback may resume
 *           detail: {item: number} // the index the play module wants the app
 *           to move to.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/skip-previous-icon-sk';
import 'elements-sk/icon/keyboard-arrow-left-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/keyboard-arrow-right-icon-sk';
import 'elements-sk/icon/skip-next-icon-sk';
import 'elements-sk/icon/video-library-icon-sk';

export type PlayMode = 'play' | 'pause';
export interface PlaySkMoveToEventDetail {
  readonly item: number
};
export interface PlaySkModeChangedManuallyEventDetail {
  readonly mode: PlayMode;
};

export class PlaySk extends ElementSk {

  private static template = (ele: PlaySk) => {
    if (ele.visual === 'simple') {
      return PlaySk.simpleTemplate(ele);
    } else {
      return PlaySk.fullTemplate(ele);
    }
  }

  private static fullTemplate = (ele: PlaySk) =>
    html`
    <div class="horizontal-flex">
      <div class='filler'></div>
      <skip-previous-icon-sk title="Go to first" @click=${ele.begin}
        ></skip-previous-icon-sk>
      <keyboard-arrow-left-icon-sk title="Step back one (,)" @click=${ele.prev}
        ></keyboard-arrow-left-icon-sk>
      ${ele._playPauseIcon(ele)}
      <keyboard-arrow-right-icon-sk title="Step forward one (.)" @click=${ele.next}
        ></keyboard-arrow-right-icon-sk>
      <skip-next-icon-sk title="Go to last" @click=${ele.end}></skip-next-icon-sk>
      <div class='filler'></div>
      <label>Delay in ms</label>
      <input value="${ele._playbackDelay}" class=delay-input @change=${ele._delayChanged}
        ></input>
    </div>`;

  private static simpleTemplate = (ele: PlaySk) =>
    html`<video-library-icon-sk title="Play/Pause" @click=${ele.togglePlay}
        id='play-button-v'></video-library-icon-sk>`;

  private _mode: PlayMode = 'pause';
  // current position in sequence
  private _item: number = 0;
  // length of sequence
  private _size: number = 2;
  // target number of milliseconds to wait between playback steps
  private _playbackDelay: number = 0;
  // time at which the last moveto event was emitted
  private _lastMoveTime: number = 0;
  // reference to a timeout we set so we can cancel it if necessary
  private _timeout: number = 0;

  /**
   * Specifies the visual style of the playback element.
   * Possible values include:
   *  'full': shows all five buttons and a textbox for controlling delay.
   *  'simple': shows only a play button, using a distinct icon.
   */
  public visual = 'full';

  constructor() {
    super(PlaySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
    if (this._timeout) window.clearTimeout(this._timeout);
  }

  get mode(): PlayMode {
    return this._mode;
  }

  // Valid values: 'play' or 'pause'
  set mode(m: PlayMode) {
    this._mode = m;
    if (m === 'play') {
      this.next();
    } else if (this._timeout) {
      console.log('paused on '+this._item);
      window.clearTimeout(this._timeout);
    }
    this._render();
  }

  get size() {
    return this._size;
  }

  set size(s: number) {
    this._size = s;
    this._item = 0;
  }

  set playbackDelay(ms: number) {
    this._playbackDelay = ms;
    this._render();
  }

  get playbackDelay() {
    return this._playbackDelay;
  }

  private _delayChanged(e: Event) {
    this._playbackDelay = parseInt((e.target as HTMLInputElement).value);
  }

  // Call this after handling the moveto event to indicate playback may proceed.
  // The application may also call this at any time to indicate it has skipped directly
  // to an item.
  movedTo(item: number) {
    this._item = item;
    if (this._mode === "play") {
      // wait out the remainder of the minimum playback delay
      const elapsed = Date.now() - this._lastMoveTime;
      const remainingMs = Math.max(0, this._playbackDelay - elapsed);
      // Must be done with timeout, even if it's zero, or we exceed call stack size
      this._timeout = window.setTimeout(() => {this.next()}, remainingMs);
    }
  }

  // template helper deciding which icon to show in the play button spot
  private _playPauseIcon(ele: PlaySk) {
    if (this._mode === 'pause') {
      return html`<play-arrow-icon-sk title="Play/Pause" @click=${ele.togglePlay}
        id='play-button'></play-arrow-icon-sk>`;
    } else {
      return html`<pause-icon-sk title="Play/Pause" @click=${ele.togglePlay}
        ></pause-icon-sk>`;
    }
  }

  togglePlay() {
    this.mode = (this._mode === 'play') ? 'pause' : 'play';
    this.dispatchEvent(
      new CustomEvent<PlaySkModeChangedManuallyEventDetail>(
        'mode-changed-manually', {
          detail: {mode: this._mode},
          bubbles: true,
        }));
  }

  // sends the moveto event
  private _triggerEvent() {
    if (this._timeout) {
      window.clearTimeout(this._timeout);
    }
    this._lastMoveTime = Date.now();
    this.dispatchEvent(
      new CustomEvent<PlaySkMoveToEventDetail>(
        'moveto', {
          detail: {item: this._item},
          bubbles: true,
        }));
  }

  begin() {
    this._item = 0;
    this._triggerEvent();
  }

  end() {
    this._item = this._size - 1;
    this._triggerEvent();
  }

  prev() {
    this._item -= 1;
    if (this._item < 0) {
      this._item += this._size;
    }
    this._triggerEvent();
  }

  next() {
    this._item = (this._item + 1) % this._size;
    this._triggerEvent();
  }
};

define('play-sk', PlaySk);
