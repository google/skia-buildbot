/** Shows the most recent binaries for which we have code size statistics. */

import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';
import {
  Binary,
  BinariesFromCommitOrPatchset,
  MostRecentBinariesRPCResponse,
} from '../rpc_types';
import '../../../infra-sk/modules/human-date-sk';

export class IndexPageSk extends ElementSk {
  private static template = (el: IndexPageSk) => html`
    <h2>Most recent code size statistics</h2>

    ${el.mostRecentBinaries === null
      ? html`<p>Loading...</p>`
      : el.mostRecentBinaries.map(
          IndexPageSk.binariesFromCommitOrPatchsetTemplate
        )}
  `;

  private static binariesFromCommitOrPatchsetTemplate = (
    binariesFromCommitOrPatchset: BinariesFromCommitOrPatchset
  ) => {
    // The metadata fields we look at should be the same for all binaries associated with this
    // commit or patchset, so we pick an arbitrary one.
    const metadata = binariesFromCommitOrPatchset.binaries[0].metadata;

    const isTryJob = metadata.patch_issue || metadata.patch_set;
    const hasDiff = metadata.bloaty_diff_args;

    const commitOrCLAnchorText = isTryJob
      ? `Issue ${metadata.patch_issue}, PS ${metadata.patch_set}`
      : metadata.revision.substring(0, 7);
    const commitOrCLAnchorHref = isTryJob
      ? `https://review.skia.org/${metadata.patch_issue}/${metadata.patch_set}`
      : `https://skia.googlesource.com/skia/+/${metadata.revision}`;

    const author = binariesFromCommitOrPatchset.binaries[0].metadata.author;
    const subject = binariesFromCommitOrPatchset.binaries[0].metadata.subject;
    const timestamp =
      binariesFromCommitOrPatchset.binaries[0].metadata.timestamp;

    // Sort the Bloaty outputs to ensure a deterministic presentation.
    binariesFromCommitOrPatchset.binaries.sort((a: Binary, b: Binary) =>
      a.metadata.binary_name < b.metadata.binary_name ||
      (a.metadata.binary_name === b.metadata.binary_name &&
        a.metadata.compile_task_name < b.metadata.compile_task_name)
        ? -1
        : 1
    );

    const hrefForBinary = (output: Binary) => {
      const params: Record<string, string> = {
        binary_name: output.metadata.binary_name,
        compile_task_name: output.metadata.compile_task_name,
      };
      if (isTryJob) {
        params.patch_issue = output.metadata.patch_issue;
        params.patch_set = output.metadata.patch_set;
      } else {
        params.commit = output.metadata.revision;
      }
      return `/binary?${new URLSearchParams(params).toString()}`;
    };

    const hrefForBinaryDiff = (output: Binary) => {
      const params: Record<string, string> = {
        binary_name: output.metadata.binary_name,
        compile_task_name: output.metadata.compile_task_name,
      };
      if (output.metadata.patch_issue) {
        // Tryjob data is identified by CL and PS
        params.patch_issue = output.metadata.patch_issue;
        params.patch_set = output.metadata.patch_set;
      } else {
        // Primary branch data is identified by git revision.
        params.commit = output.metadata.revision;
      }
      return `/binary_diff?${new URLSearchParams(params).toString()}`;
    };

    return html`
      <div class="commit-or-cl">
        <p>
          <a href="${commitOrCLAnchorHref}">${commitOrCLAnchorText}</a>
          ${subject}
          <br />
          <span class="author-and-timestamp">
            ${author},
            <human-date-sk .date=${timestamp} .diff=${true}></human-date-sk>
            ago.
          </span>
        </p>
        <p class="binaries">Binaries:</p>
        <table>
          ${binariesFromCommitOrPatchset.binaries.map(
            (output) => html`
              <tr>
                <td>
                  <a href="${hrefForBinary(output)}"
                    >${output.metadata.binary_name}</a
                  >
                </td>
                <td>
                  <a
                    href="https://task-scheduler.skia.org/task/${output.metadata
                      .task_id}">
                    ${output.metadata.compile_task_name}
                  </a>
                </td>
                ${hasDiff
                  ? html`<td>
                      <a href="${hrefForBinaryDiff(output)}">Size Diff</a>
                    </td>`
                  : ''}
              </tr>
            `
          )}
        </table>
      </div>
    `;
  };

  private mostRecentBinaries: BinariesFromCommitOrPatchset[] | null = null;

  constructor() {
    super(IndexPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.loadBinaries();
  }

  private async loadBinaries(): Promise<void> {
    // Show a loading indicator while waiting for the RPC to complete.
    const res = await CodesizeScaffoldSk.waitFor(
      fetch('/rpc/most_recent_binaries/v1')
        .then(jsonOrThrow)
        .then((r: MostRecentBinariesRPCResponse) => r)
    );
    this.mostRecentBinaries = res.binaries;
    this._render();
  }
}
define('index-page-sk', IndexPageSk);
