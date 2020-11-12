/**
 * @module modules/android-layers-sk
 * @description <h2><code>android-layers-sk</code></h2>
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LayerInfo, CommandsSkJumpEventDetail } from '../commands-sk/commands-sk'


// Types for the wasm bindings
import { LayerSummary } from '../debugger';

import '../cycler-button-sk';

export interface LayerDescription {
  nodeId: number,
  frameOfLastUpdate: number,
  fullRedraw: boolean,
  layerWidth: number,
  layerHeight: number
  name: string,
  // A list of indices of drawImageRectLayer commands that reference this layer
  usesThisFrame: number[],
  updatedThisFrame: boolean,
}

// An event to trigger the inspector for a given layer update.
// A layer update is fully specified by the node id and a frame on which an
// update to it occurred.
export interface AndroidLayersSkInspectLayerEventDetail {
  id: number;
  frame: number;
}

export class AndroidLayersSk extends ElementSk {

  private static template = (ele: AndroidLayersSk) =>
    html`<div>${ele._layerList.map(
      (l: LayerDescription) => AndroidLayersSk.layerTemplate(ele, l))}
      </div>`;

  private static layerTemplate = (ele: AndroidLayersSk, item: LayerDescription) =>
    html`
    <div class="androidlayerbox ${item.nodeId === ele._inspectedLayer ? 'selected' : ''}">
      <span class="layername"><b>${item.nodeId}</b>: ${item.name}</span><br>
      Layer size = <b>(${item.layerWidth}, ${item.layerHeight})</b><br>
      Uses this frame = <b>${item.usesThisFrame.length}</b>
      Last update (<b>${item.fullRedraw ? 'full' : 'partial'}</b>) on frame
        <b>${item.frameOfLastUpdate}</b><br>
      <cycler-button-sk .text=${'Show Use'} .list=${item.usesThisFrame} .fn=${ele._jumpCommand}
        title="Cycle through drawImageRectLayer commands on this frame which used this surface as\
 a source.">
      </cycler-button-sk>
      <button @click=${()=>ele._inspectLayer(item.nodeId, item.frameOfLastUpdate)}
        class="${item.nodeId === ele._inspectedLayer ? 'buttonselected' : ''}"
        title="Open the SkPicture representing the update on frame ${item.frameOfLastUpdate}.">
        ${item.nodeId === ele._inspectedLayer
          ? 'Exit'
          : 'Inspector'
        }
      </button>
    </div>
    `;

  private _layerList: LayerDescription[] = [];
  private _inspectedLayer: number = -1; // a nodeID, not an index

  constructor() {
    super(AndroidLayersSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // Given layer info (maps from ids to names and uses) collected in processCommands
  // and summaries from wasm, (info about which past frames contain info necessary to
  // redraw a layer's offscreen buffer), Create the array used to draw the template.
  update(maps: LayerInfo, summaries: LayerSummary[], frame: number) {
    this._layerList = [];
    summaries.forEach((item: LayerSummary) => {
      const ld: LayerDescription = {
        nodeId: item.nodeId,
        frameOfLastUpdate: item.frameOfLastUpdate,
        fullRedraw: item.fullRedraw,
        layerWidth: item.layerWidth,
        layerHeight: item.layerHeight,
        name: maps.names.get(item.nodeId)!,
        usesThisFrame: maps.uses.get(item.nodeId) || [],
        updatedThisFrame: item.frameOfLastUpdate === frame,
      };
      // We only want to see it if it's updated or used this frame.
      if (ld.updatedThisFrame || ld.usesThisFrame.length > 0) {
        this._layerList.push(ld);
      }
    });
    this._render();
  }

  private _inspectLayer(id: number, frame: number) {
    if (this._inspectedLayer === id) {
      id = -1; // means we are not inspecting any layer
    }
    this._inspectedLayer = id;
    // The current frame must be set to one which has an update for a layer before opening
    // the inspector for that layer. debugger-page-sk will move the frame if necessary.
    this.dispatchEvent(
    new CustomEvent<AndroidLayersSkInspectLayerEventDetail>(
      'inspect-layer', {
        detail: {id: this._inspectedLayer, frame: frame},
        bubbles: true,
      }));
    this._render();
  }

  private _jumpCommand(index: number) {
    this.dispatchEvent(
      new CustomEvent<CommandsSkJumpEventDetail>(
        'jump-command', {
          detail: {unfilteredIndex: index},
          bubbles: true,
        }));
  }
};

define('android-layers-sk', AndroidLayersSk);
