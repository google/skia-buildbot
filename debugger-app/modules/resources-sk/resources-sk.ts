/**
 * @module modules/resources-sk
 * @description A view of the shared images that are present in an MSKP file.
 *  Contains a scrollable area suitable for viewing images with transparency
 *  and an image selection function so different parts of the app can send you
 *  to an image here, or you can pick one and view details about it.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { DebuggerPageSkLightDarkEventDetail } from '../debugger-page-sk/debugger-page-sk';
import { CommandsSkJumpEventDetail } from '../commands-sk/commands-sk';
import { TimelineSkMoveFrameEventDetail } from '../timeline-sk/timeline-sk';

import { SkpDebugPlayer } from '../debugger';

interface ImageItem {
  // The image indices provided from the debugger are contiguous
  index: number;
  width: number;
  height: number;
  pngUri: string;
  // A map from commands to lists of frames
  uses: Map<number, number[]>;
}

export class ResourcesSk extends ElementSk {
  private static template = (ele: ResourcesSk) =>
    html`
      <p>${ele._list.length} images were stored in this file. Note that image indices here are
         file indices, which corresponded 1:1 to the gen ids that the images had during recording.
         If an image appears more than once, that indicates there were multiple copies of it in
         memory at record time. These indices appear in the 'imageIndex' field of commands using
         them. This metadata is only recorded for multi frame (mskp) files from android, so
         nothing is shown here for other skps.
      </p>
      <div class="main-box ${ ele._backdropStyle }">
        ${ele._list.map((item) => ResourcesSk.templateImage(ele, item))}
      </div>
      <div class="selection-detail">
        Selected: ${ ele._selection !== null
          ? ResourcesSk.templateSelectionDetail(ele, ele._list[ele._selection])
          : 'none'
        }
      </div>`;

  private static templateImage = (ele: ResourcesSk, item: ImageItem) =>
    html`
      <div class="image-box">
        <span class="resource-name ${ele._textContrast()}">
          ${ele._displayName(item)}
        </span><br>
        <img src="${ item.pngUri }" id="res-img-${item.index}"
          class="outline-on-hover ${ ele._selection === item.index ? 'selected-image' : '' }"
          @click=${()=>{ele.selectItem(item.index)}}/>
      </div>`;

  private static templateSelectionDetail = (ele: ResourcesSk, item: ImageItem) =>
    html`
      <b>${ item.index }</b><br>
      size: (${ item.width }, ${ item.height })<br>
      Usage: ${ item.uses.size > 0
        ? html`<table class="usage-table">${ ele._usageTable(item) }</table>`
        : html`<p>No uses found in any drawImage* commands. May occur in shaders.</p>`
      }`;

  private _list: ImageItem[] = new Array<ImageItem>();
  private _selection: number | null = null;
  private _backdropStyle = 'light-checkerboard';

  constructor() {
    super(ResourcesSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    document.addEventListener('light-dark', (e) => {
      this._backdropStyle = (e as CustomEvent<DebuggerPageSkLightDarkEventDetail>).detail.mode;
      this._render();
    });
  }

  // Load resource data for current file from player and build the array that
  // drives the resource grid template
  // To be called once after file load.
  // resources-sk will not save any reference to the player.
  update(player: SkpDebugPlayer) {
    // At the time of this writing, only MSKP files recorded on android have the necessary metadata
    // to show shared image use across the file.

    const imageCount = player.getImageCount();
    this._list = [];
    for (var i = 0; i < imageCount; i++) {
      const info = player.getImageInfo(i);
      this._list.push({
        index: i,
        width: info.width,
        height: info.height,
        pngUri: player.getImageResource(i),
        uses: new Map<number, number[]>(), // this will be populated below
      });
    }

    for (let fp = 0; fp < player.getFrameCount(); fp++) {
      const oneFrameUseMap = player.imageUseInfoForFrameJs(fp);
      for (const [imageIdStr, listOfCommands] of Object.entries(oneFrameUseMap)) {
        const id = parseInt(imageIdStr);
        for (const com of (listOfCommands as number[])) {
          if (!(this._list[id].uses.has(com))) {
            this._list[id].uses.set(com, []);
          }
          this._list[id].uses.get(com)!.push(fp);
        }
      }
    }

    this._render();
  }

  selectItem(i: number, scroll=false) {
    this._selection = i;
    this._render();
    if (scroll) {
      this.querySelector<HTMLImageElement>('#res-img-' + i
        )?.scrollIntoView({block: 'nearest'});
    }
  }

  private _displayName(item: ImageItem): string {
    return `${ item.index } (${ item.width }, ${ item.height })`
  }

  private _textContrast() {
    if (this._backdropStyle === 'light-checkerboard') {
      return 'dark-text';
    } else {
      return 'light-text';
    }
  }

  private _usageTable(item: ImageItem) {
    const out = new Array();
    item.uses.forEach((frames: number[], key: number) => {
      out.push(html`
      <tr>
        <td class="leftmost-cell"> Command <b>${ key }</b> on frames: </td>
        ${ frames.map((f : number) => html`
          <td title="Jump to command ${ key } frame ${ f }"
            @click=${()=>{this._jump(f, key)}}>${ f }</td>`) }
      </tr>`)
    })
    return out;
  }

  private _jump(frame: number, command: number) {
    console.log(`Jump to command ${ command } frame ${ frame }`);
    this.dispatchEvent(
      new CustomEvent<TimelineSkMoveFrameEventDetail>(
        'set-frame', {
          detail: {frame: frame},
          bubbles: true,
        }));
    this.dispatchEvent(
      new CustomEvent<CommandsSkJumpEventDetail>(
        'jump-command', {
          detail: {unfilteredIndex: command},
          bubbles: true,
        }));
  }
};

define('resources-sk', ResourcesSk);
