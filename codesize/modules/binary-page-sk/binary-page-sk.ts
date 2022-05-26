/** Shows code size statistics about a single binary. */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { load } from '@google-web-components/google-chart/loader';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';
import { BloatyOutputMetadata, BinaryRPCRequest, BinaryRPCResponse } from '../rpc_types';
import '../../../infra-sk/modules/human-date-sk';

import '@google-web-components/google-chart/';

export class BinaryPageSk extends ElementSk {
  private static template = (el: BinaryPageSk) => {
    if (el.metadata === null) {
      return html`<p>Loading...</p>`;
    }

    const isTryJob = el.metadata?.patch_issue || el.metadata?.patch_set;
    const commitOrCLAnchorText = isTryJob
      ? `Issue ${el.metadata?.patch_issue}, PS ${el.metadata?.patch_set}`
      : el.metadata?.revision.substring(0, 7);
    const commitOrCLAnchorHref = isTryJob
      ? `https://review.skia.org/${el.metadata?.patch_issue}/${el.metadata?.patch_set}`
      : `https://skia.googlesource.com/skia/+/${el.metadata?.revision}`;

    const compileTaskNameHref = `https://task-scheduler.skia.org/task/${el.metadata?.task_id}`;
    return html`
      <h2>
        Code size statistics for <code>${el.metadata?.binary_name}</code>
        <span class="compile-task">
          (<a href="${compileTaskNameHref}">${el.metadata?.compile_task_name}</a>)
        </span>
      </h2>

      <p>
        <a href="${commitOrCLAnchorHref}">${commitOrCLAnchorText}</a>
        ${el.metadata?.subject}
        <br/>
        <span class="author-and-timestamp">
          ${el.metadata?.author},
          <human-date-sk .date=${el.metadata?.timestamp} .diff=${true}></human-date-sk> ago.
        </span>
      </p>

      <p class="instructions">Instructions:</p>

      <ul>
        <li><strong>Click</strong> on a node to navigate down the tree.</li>
        <li><strong>Right click</strong> anywhere on the treemap go back up one level.</li>
      </ul>

      <div id="treemap"></div>
    `;
  }

  private metadata: BloatyOutputMetadata | null = null;

  constructor() {
    super(BinaryPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Show a loading indicator while the tree is loading.
    CodesizeScaffoldSk.waitFor(this.loadTreeMap());
  }

  private async loadTreeMap(): Promise<void> {
    const params = new URLSearchParams(window.location.search);
    const request: BinaryRPCRequest = {
      commit: params.get('commit') || '',
      patch_issue: params.get('patch_issue') || '',
      patch_set: params.get('patch_set') || '',
      binary_name: params.get('binary_name') || '',
      compile_task_name: params.get('compile_task_name') || '',
    };

    const [, response] = await Promise.all([
      load({ packages: ['treemap'] }),
      fetch('/rpc/binary/v1', { method: 'POST', body: JSON.stringify(request) })
        .then(jsonOrThrow)
        .then((r: BinaryRPCResponse) => r),
    ]);

    this.metadata = response.metadata;
    this._render();

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
      return `<div style="background:#0a3055; padding:10px; border-style:solid">
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
define('binary-page-sk', BinaryPageSk);
