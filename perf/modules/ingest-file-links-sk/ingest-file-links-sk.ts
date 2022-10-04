/**
 * @module modules/ingest-file-links-sk
 * @description <h2><code>ingest-file-links-sk</code></h2>
 *
 * Displays links, if any are found, in the ingest.Format
 * for the provided CommitNumber and traceID.
 *
 * See also https://pkg.go.dev/go.skia.org/infra/perf/go/ingest/format#Format
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CommitDetailsRequest, CommitNumber, ingest } from '../json/all';

export class IngestFileLinksSk extends ElementSk {
  private links: { [key: string]: string } | null = null;

  private spinner: SpinnerSk | null = null;

  constructor() {
    super(IngestFileLinksSk.template);
  }

  private static displayLinks = (ele: IngestFileLinksSk): TemplateResult[] => {
    const keys = Object.keys(ele.links || {}).sort();
    return keys.map((key) => html`<li><a href="${ele.links![key]}">${key}</a></li>`);
  }

  private static template = (ele: IngestFileLinksSk) => html`
  <div ?hidden=${Object.keys(ele.links || {}).length === 0}>
    <h3> Links</h3>
    <spinner-sk id="spinner"></spinner-sk>
    <ul>
      ${IngestFileLinksSk.displayLinks(ele)}
    </ul>
  </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.spinner = $$('#spinner', this);
  }

  // load and display the links for the given commit and trace.
  public async load(cid: CommitNumber, traceid: string): Promise<void> {
    if (this.spinner!.active === true) {
      return;
    }
    const body: CommitDetailsRequest = {
      cid: cid,
      traceid: traceid,
    };
    this.spinner!.active = true;
    const url = '/_/details/?results=false';

    this.links = {};
    try {
      const resp = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      });
      const json = await jsonOrThrow(resp);
      const format = json as ingest.Format;
      if (!format.version) {
        // Bail out if results were stored in the legacy format, which doesn't set
        // a value for 'version'.
        return;
      }
      this.links = format.links!;
    } catch (error) {
      await errorMessage(error as string);
    } finally {
      this.spinner!.active = false;
      this._render();
    }
  }
}

define('ingest-file-links-sk', IngestFileLinksSk);
