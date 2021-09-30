/**
 * @module modules/resources-sk
 * @description A view of the shared images that are present in an MSKP file.
 *  Contains a scrollable area suitable for viewing images with transparency
 *  and an image selection function so different parts of the app can send you
 *  to an image here, or you can pick one and view details about it.
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementDocSk } from '../element-doc-sk/element-doc-sk';
import { DefaultMap } from '../default-map';

import { SkpDebugPlayer } from '../debugger';
import {
  JumpCommandEventDetail,
  JumpCommandEvent,
  JumpInspectLayerEvent,
  ToggleBackgroundEventDetail, InspectLayerEventDetail, ToggleBackgroundEvent, BackgroundStyle,
} from '../events';

interface ImageItem {
  // The image indices provided from the debugger are contiguous
  index: number;
  width: number;
  height: number;
  pngUri: string;
  // A map from commands to lists of frames
  uses: DefaultMap<number, number[]>;
  // An image use map for every layer id.
  layeruses: DefaultMap<number, DefaultMap<number, number[]>>;
}

function displayName(item: ImageItem): string {
  return `${item.index} (${item.width}, ${item.height})`;
}

export class ResourcesSk extends ElementDocSk {
  private static template = (ele: ResourcesSk) => html`
      <p>${ele.list.length} images were stored in this file. Note that image indices here are
         file indices, which corresponded 1:1 to the gen ids that the images had during recording.
         If an image appears more than once, that indicates there were multiple copies of it in
         memory at record time. These indices appear in the 'imageIndex' field of commands using
         them. This metadata is only recorded for multi frame (mskp) files from android, so
         nothing is shown here for other skps. All images in SKPs are serialized as PNG regardless
         of their original encoding.
      </p>
      <div class="main-box ${ele.backdropStyle}">
        ${ele.list.map((item: ImageItem) => ResourcesSk.templateImage(ele, item))}
      </div>
      <div class="selection-detail">
        Selected: ${ele.selection !== null
    ? ResourcesSk.templateSelectionDetail(ele, ele.list[ele.selection])
    : 'none'
        }
      </div>`;

  private static templateImage = (ele: ResourcesSk, item: ImageItem) => html`
      <div class="image-box">
        <span class="resource-name ${ele.textContrast()}"
         @click=${() => { ele.selectItem(item.index); } /* makes it easier to select 1x1 images */}>
          ${displayName(item)}
        </span><br>
        <img src="${item.pngUri}" id="res-img-${item.index}"
          class="outline-on-hover ${ele.selection === item.index ? 'selected-image' : ''}"
          @click=${() => { ele.selectItem(item.index); }}/>
      </div>`;

  private static templateSelectionDetail = (ele: ResourcesSk, item: ImageItem) => html`
      <b>${item.index}</b><br>
      size: (${item.width}, ${item.height})<br>
      Usage in top-level skp
      ${item.uses.size > 0
    ? html`<table class="usage-table">${ele.usageTable(item.uses)}</table>`
    : html`<p>No uses found in any drawImage* commands. May occur in shaders.</p>`
      }
      Usage in offscreen buffers
      ${item.layeruses.size > 0
        ? html`<table class="usage-table">${ele.layerUsageTable(item.layeruses)}</table>`
        : html`<p>Not used</p>`
      }`;

  private list: ImageItem[] = [];

  private selection: number | null = null;

  private backdropStyle: BackgroundStyle = 'light-checkerboard';

  constructor() {
    super(ResourcesSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.addDocumentEventListener(ToggleBackgroundEvent, (e) => {
      this.backdropStyle = (e as CustomEvent<ToggleBackgroundEventDetail>).detail.mode;
      this._render();
    });
  }

  reset(): void {
    this.list = [];
    this.selection = null;
    this._render();
  }

  // Load resource data for current file from player and build the array that
  // drives the resource grid template
  // To be called once after file load.
  // resources-sk will not save any reference to the player.
  update(player: SkpDebugPlayer): void {
    // At the time of this writing, only MSKP files recorded on android have the necessary metadata
    // to show shared image use across the file.

    const imageCount = player.getImageCount();
    this.list = [];
    for (let i = 0; i < imageCount; i++) {
      const info = player.getImageInfo(i);
      this.list.push({
        index: i,
        width: info.width,
        height: info.height,
        pngUri: player.getImageResource(i),
        // this will be populated below
        uses: new DefaultMap<number, number[]>(() => []),
        // one use map for every layer.
        layeruses: new DefaultMap<number, DefaultMap<number, number[]>>(
          () => new DefaultMap<number, number[]>(() => []),
        ),
      });
    }

    // Collect uses at top level
    for (let fp = 0; fp < player.getFrameCount(); fp++) {
      const oneFrameUseMap = player.imageUseInfo(fp, -1);
      for (const [imageIdStr, listOfCommands] of Object.entries(oneFrameUseMap)) {
        const id = parseInt(imageIdStr, 10);
        for (const com of listOfCommands) {
          this.list[id].uses.get(com).push(fp);
        }
      }
    }

    // collect uses for every layer
    const keys = player.getLayerKeys();
    for (const key of keys) {
      const useMap = player.imageUseInfo(key.frame, key.nodeId);
      for (const [imageIdStr, listOfCommands] of Object.entries(useMap)) {
        const id = parseInt(imageIdStr, 10);
        for (const com of listOfCommands) {
          this.list[id].layeruses.get(key.nodeId).get(com).push(key.frame);
        }
      }
    }

    this._render();
  }

  selectItem(i: number, scroll: boolean = false): void {
    this.selection = i;
    this._render();
    if (scroll) {
      this.querySelector<HTMLImageElement>(`#res-img-${i}`)?.scrollIntoView({ block: 'nearest' });
    }
  }

  private textContrast() {
    if (this.backdropStyle === 'light-checkerboard') {
      return 'dark-text';
    }
    return 'light-text';
  }

  // Supply a non-negative nodeId to make jump actions go to a particular layer
  private usageTable(uses: Map<number, number[]>, nodeId: number = -1): TemplateResult[] {
    const out: TemplateResult[] = [];
    uses.forEach((frames: number[], key: number) => {
      out.push(html`
      <tr>
        <td class="command-cell"> Command <b>${key}</b> on frames: </td>
        ${frames.map((f: number) => html`
          <td title="Jump to command ${key} frame ${f} ${
  nodeId >= 0 ? `on layer ${nodeId}` : ''
}"
            class="clickable-cell"
            @click=${() => { this.jump(f, key, nodeId); }}>${f}</td>`)}
      </tr>`);
    });
    return out;
  }

  private layerUsageTable(layeruses: DefaultMap<number, DefaultMap<number, number[]>>): TemplateResult[] {
    const out: TemplateResult[] = [];
    layeruses.forEach((usemap: DefaultMap<number, number[]>, nodeid: number) => {
      out.push(html`
      <tr><td class="layer-cell"><b>Layer ${nodeid}</b></td></tr>
      ${this.usageTable(usemap, nodeid)}`);
    });
    return out;
  }

  private jump(frame: number, command: number, nodeId: number): void {
    // Note that the app may already be in the inspector for a layer.
    // even for non-layer jumps, this is an appropriate way to get there,
    // becuase it will close the inspector if needed.
    // debugger-page-sk will move the frame when handling this event.
    this.dispatchEvent(
      new CustomEvent<InspectLayerEventDetail>(
        JumpInspectLayerEvent, {
          detail: { id: nodeId, frame: frame },
          bubbles: true,
        },
      ),
    );
    this.dispatchEvent(
      new CustomEvent<JumpCommandEventDetail>(
        JumpCommandEvent, {
          detail: { unfilteredIndex: command },
          bubbles: true,
        },
      ),
    );
  }
}

define('resources-sk', ResourcesSk);
