/** Shows a size diff between two binaries. */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';
import { BloatyOutputMetadata, BinarySizeDiffRPCResponse, BinarySizeDiffRPCRequest } from '../rpc_types';
import '../../../infra-sk/modules/human-date-sk';

export class BinaryDiffPageSk extends ElementSk {
  private static template = (el: BinaryDiffPageSk) => {
    if (el.metadata === null) {
      return html`<p>Loading...</p>`;
    }

    const clAnchorText = `Issue ${el.metadata?.patch_issue}, PS ${el.metadata?.patch_set}`;
    const clAnchorHref = `https://review.skia.org/${el.metadata?.patch_issue}/${el.metadata?.patch_set}`;

    const compileTaskNameHref = `https://task-scheduler.skia.org/task/${el.metadata?.task_id}`;
    return html`
      <h2>
        Binary size diff for <code>${el.metadata?.binary_name}</code>
        <span class="compile-task">
          (<a href="${compileTaskNameHref}">${el.metadata?.compile_task_name}</a>
          vs
          <a href="${compileTaskNameHref}">${el.metadata?.compile_task_name_no_patch}</a>)
        </span>
      </h2>

      <p>
        <a href="${clAnchorHref}">${clAnchorText}</a>
        ${el.metadata?.subject}
        <br/>
        <span class="author-and-timestamp">
          ${el.metadata?.author},
          <human-date-sk .date=${el.metadata?.timestamp} .diff=${true}></human-date-sk> ago.
        </span>
      </p>

      <pre>${el.raw_diff}</pre>

      <p>
        <strong>Note:</strong> Some small spurious deltas may occur due to differences in which the
        two binaries are compiled. See
        <a href="https://skia-review.googlesource.com/c/skia/+/556358">this CL</a>'s description for more.
      </p>
    `;
  }

  private metadata: BloatyOutputMetadata | null = null;

  private raw_diff: string | null = null;

  constructor() {
    super(BinaryDiffPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Show a loading indicator while the RPC is in progress.
    CodesizeScaffoldSk.waitFor(this.loadDiff());
  }

  private async loadDiff(): Promise<void> {
    const params = new URLSearchParams(window.location.search);
    const request: BinarySizeDiffRPCRequest = {
      commit: params.get('commit') || '',
      patch_issue: params.get('patch_issue') || '',
      patch_set: params.get('patch_set') || '',
      binary_name: params.get('binary_name') || '',
      compile_task_name: params.get('compile_task_name') || '',
    };
    const response = await fetch('/rpc/binary_size_diff/v1', { method: 'POST', body: JSON.stringify(request) })
      .then(jsonOrThrow)
      .then((r: BinarySizeDiffRPCResponse) => r);

    this.metadata = response.metadata;
    this.raw_diff = response.raw_diff;
    this._render();
  }
}
define('binary-diff-page-sk', BinaryDiffPageSk);
