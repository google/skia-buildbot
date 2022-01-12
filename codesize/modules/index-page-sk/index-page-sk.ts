/** Home page of codesize.skia.org. */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { load } from '@google-web-components/google-chart/loader';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../codesize-scaffold-sk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';
import { BloatyRPCResponse } from '../rpc_types';

import '@google-web-components/google-chart/';

export class IndexPageSk extends ElementSk {
  private static template = (el: IndexPageSk) => html`
    <codesize-scaffold-sk>
      <!--
        TODO(lovisolo): The artifact name should be determined from metadata returned by the RPC.
      -->
      <h2>Debug build of the <code>dm</code> binary</h2>

      <p>Instructions:</p>

      <ul>
        <li><strong>Click</strong> on a node to navigate down the tree.</li>
        <li><strong>Right click</strong> anywhere on the treemap go back up one level.</li>
      </ul>

      <div id="treemap"></div>
    </codesize-scaffold-sk>
  `;

  constructor() {
    super(IndexPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Show a loading indicator while the tree is loading.
    CodesizeScaffoldSk.waitFor(this.loadTreeMap());
  }

  private async loadTreeMap(): Promise<void> {
    const [, response] = await Promise.all([
      load({ packages: ['treemap'] }),
      fetch('/rpc/bloaty/v1').then((r) => r.json() as Promise<BloatyRPCResponse>),
    ]);

    const rows = [
      ['Name', 'Parent', 'Size'],
      ...response.rows.map((row) => [
        row.name,
        // The RPC represents empty parents as the empty string, but TreeMap expects a null value.
        row.parent || null,
        row.size,
      ]),
    ];
    const data = google.visualization.arrayToDataTable(rows);
    const tree = new google.visualization.TreeMap(this.querySelector('#treemap')!);

    const showTooltip = (row: number, size: string) => {
      const escapedLabel = data.getValue(row, 0)
        .replace('&', '&amp;')
        .replace('<', '&lt;')
        .replace('>', '&gt;');
      return `<div style="background:#fd9; padding:10px; border-style:solid">
              <span style="font-family:Courier"> ${escapedLabel} <br>
              Size: ${size} </div>`;
    };

    // For some reason the type definition for TreeMapOptions does not include the generateTooltip
    // option (https://developers.google.com/chart/interactive/docs/gallery/treemap#tooltips), so
    // a type assertion is necessary to keep the TypeScript compiler happy.
    const options = {
      generateTooltip: showTooltip,
    } as google.visualization.TreeMapOptions;

    // Draw the tree and wait until the tree finishes drawing.
    await new Promise((resolve) => {
      google.visualization.events.addOneTimeListener(tree, 'ready', resolve);
      tree.draw(data, options);
    });
  }
}
define('index-page-sk', IndexPageSk);
