/**
 * @module modules/timeline-sk
 * @description An element for displaying a position in a timeline.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { PlaySk, PlaySkMoveToEventDetail } from '../play-sk/play-sk'

import '../play-sk';

export interface TimelineSkMoveFrameEventDetail {
  frame: number,
}

export class TimelineSk extends ElementSk {
  private static template = (ele: TimelineSk) =>
    html`
    <div class='horizontal'>
      <play-sk .visual=${'simple'}></play-sk>
      <div class="outer">
        <div class="hallway">
          ${ [...Array(ele._count).keys()].map((i) => html`
            <div class="room ${ ele._item == i
              ? 'selected'
              : 'not-selected'
              }" @click=${() => { ele._roomClick(i); }}>${ (i % ele._modulo) == 0
                ? html`<div class="rel-point">
                    <span class="label">${ i }<span>
                  </div>`
                : ''
              }</div>
          `) }
        </div>
        <div class="basement"><div>
      </div>
    </div>`;

  private _count = 50;
  private _item = 0;
  private _modulo = 5;
  // Play submodule
  private _playSk: PlaySk | null = null;

  set item(i: number) {
    this._item = i;
    this._render();
    // notify debugger-page-sk to change the frame
    // TODO(nifong): the timeline element always appears to be one frame late during playback
    // this could be fixed by deferring this event with window.setTimeout, but that would break
    // pretty much any other code that sets timeline.item, such as the inspect button.
    this.dispatchEvent(
    new CustomEvent<TimelineSkMoveFrameEventDetail>(
      'move-frame', {
        detail: {frame: this._item},
        bubbles: true,
      }));

  }

  set count(c: number) {
    this._count = c;
    this._playSk!.mode = 'pause';
    this._playSk!.size = this._count;
    const hallway = this.querySelector<HTMLElement>('div.hallway')!;
    const strW = window.getComputedStyle(hallway, null).width;
    const width = parseFloat(strW.substring(0, strW.length-2));
    const space = 70; // minimum pixels of space to give each label.
    this._modulo = Math.ceil(space * this._count / width);
    this._render();
  }

  get playsk(): PlaySk {
    return this._playSk!;
  }

  constructor() {
    super(TimelineSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this._playSk = this.querySelector<PlaySk>('play-sk');
    this._playSk!.size = this._count;

    this._playSk!.addEventListener('moveto', (e) => {
      this.item = (e as CustomEvent<PlaySkMoveToEventDetail>).detail.item;
    });

    this._render();
  }

  private _roomClick(i: number) {
    this._playSk!.mode = 'pause';
    this.item = i;
  }
};

define('timeline-sk', TimelineSk);
