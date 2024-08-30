/**
 * @module modules/ingest-file-links-sk
 * @description <h2><code>ingest-file-links-sk</code></h2>
 *
 * Displays links, if any are found, in the ingest.Format
 * for the provided CommitNumber and traceID.
 *
 * See also https://pkg.go.dev/go.skia.org/infra/perf/go/ingest/format#Format
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { $$ } from '../../../infra-sk/modules/dom';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CommitDetailsRequest, CommitNumber, ingest } from '../json';

function isUrl(link: string): boolean {
  try {
    // eslint-disable-next-line no-new
    new URL(link);
    return true;
  } catch (e) {
    return false;
  }
}

export class IngestFileLinksSk extends ElementSk {
  private links: { [key: string]: string } | null = null;

  private spinner: SpinnerSk | null = null;

  constructor() {
    super(IngestFileLinksSk.template);
  }

  private static displayLinks = (ele: IngestFileLinksSk): TemplateResult[] => {
    const keys = Object.keys(ele.links || {}).sort();
    const getHtml = (key: string): TemplateResult => {
      const link = ele.links![key];
      if (isUrl(link)) {
        return html`<li><a href="${link}">${key}</a></li>`;
      }
      return html`<li>${key}: ${link}</li>`;
    };

    return keys.map(getHtml);
  };

  private static template = (ele: IngestFileLinksSk) => html`
    <div ?hidden=${Object.keys(ele.links || {}).length === 0}>
      <h3>Links</h3>
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
