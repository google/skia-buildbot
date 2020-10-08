/**
 * @module modules/play-sk
 * @description A playback controller for looping over a sequence.
 *
 * @evt mode-changed-manually - After the user clicks the play/pause button
 *          (but not if you set the mode property from code)
 *           detail: {mode: string} // 'play' or 'pause'
 * @evt moveto - Indicating that the playback module wants to move the application to a new item
 *           The application should handle the event by updating itself to show the new item, then
 *           call movedTo(item) when finished to indicate playback may resume
 *           detail: {item: number}
 *
 * @example
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

export class PlaySk extends ElementSk {
  private static template = (ele: PlaySk) =>
    html`
    <div class="horizontal-flex">
      <skip-previous-icon-sk        title="Go to first"          @click=${ele._begin}     ></skip-previous-icon-sk>
      <keyboard-arrow-left-icon-sk  title="Step back one (,)"    @click=${ele._prev}      ></keyboard-arrow-left-icon-sk>
      ${ele._playPauseIcon(ele)}
      <keyboard-arrow-right-icon-sk title="Step forward one (.)" @click=${ele._next}      ></keyboard-arrow-right-icon-sk>
      <skip-next-icon-sk            title="Go to last"           @click=${ele._end}       ></skip-next-icon-sk>
      <label>Delay in ms</label>
      <input value="${ele._playbackDelay}" class=delay-input></input>
    </div>`;

  private _mode: string = 'pause';
  // current position in sequence
  private _item: number = 0;
  // length of sequence
  private _size: number = 2;
  // target number of milliseconds to wait between playback steps
  private _playbackDelay: number = 0;
  // time at which the last moveto event was emitted
  private _lastMoveTime: number = 0;
  // reference to a timeout we set so we can cancel it if necessary
  private _timeout: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    super(PlaySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get mode() {
    return this._mode;
  }

  // Valid values: 'play' or 'pause'
  set mode(m: string) {
    this._mode = m;
    if (m === 'play') {
      this._nextItem();
    } else if (this._timeout) {
      clearTimeout(this._timeout);
    }
    this._render();
  }

  get size() {
    return this._size;
  }

  // Valid values: 'play' or 'pause'
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

  // Call this after handling the moveto event to indicate playback may proceed.
  // The application may also call this at any time to indicate it has skipped directly to an item.
  movedTo(item: number) {
    this._item = item;
    if (this._mode == "play") {
      // wait out the remainder of the minimum playback delay
      const elapsed = Date.now() - this._lastMoveTime;
      const remainingMs = this._playbackDelay - elapsed;
      if (remainingMs <= 0) {
        this._nextItem();
      } else {
        this._timeout = setTimeout(() => {this._nextItem()}, remainingMs);
      }
    }
  }

  // template helper deciding which icon to show in the play button spot
  _playPauseIcon(ele: PlaySk) {
    if (this._mode === 'pause') {
      return html`<play-arrow-icon-sk title="Play/Pause" @click=${ele._togglePlay}></play-arrow-icon-sk>`;
    } else {
      return html`<pause-icon-sk title="Play/Pause" @click=${ele._togglePlay}></pause-icon-sk>`;
    }
  }

  _togglePlay() {
    console.log('_togglePlay');
    this._mode = (this._mode == "play") ? "pause" : "play";
    this._render();
    this.dispatchEvent(new CustomEvent('mode-changed-manually',
      { detail: {mode: this._mode}, bubbles: true }));
  }

  // sends the moveto event
  _triggerEvent() {
    if (this._timeout) {
      clearTimeout(this._timeout);
    }
    this._lastMoveTime = Date.now();
    this.dispatchEvent(new CustomEvent('moveto',
      { detail: {item: this._item}, bubbles: true }));
  }

  _nextItem() {
    this._item = (this._item+1) % this._size;
    this._triggerEvent();
  }

  _begin() {
    this._item = 0;
    this._triggerEvent();
  }

  _end() {
    this._item = this._size-1;
    this._triggerEvent();
  }

  _prev() {
    this._item -= 1;
    if (this._item < 0) {
      this._item += this._size;
    }
    this._triggerEvent();
  }

  _next() {
    this._nextItem();
  }
};

define('play-sk', PlaySk);
