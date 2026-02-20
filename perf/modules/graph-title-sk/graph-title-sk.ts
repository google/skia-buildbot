/**
 * @module modules/graph-title-sk
 * @description <h2><code>graph-title-sk</code></h2>
 *
 * A Title element meant to be specifically used for individual Graphs. A map of title entries
 * can be provided through the set function. The key value pairs are displayed in two separate
 * rows.
 *
 * @example
 *
 * Input:
 * {
 *  "bot": "linux-perf",
 *  "benchmark": "Speedometer2",
 *  "test": "",
 *  "subtest_1": "100_objects_allocated_at_initialization"
 * }
 *
 * Output raw title:
 * bot          benchmark     subtest_1
 * linux-perf   Speedometer2  100_objects_allocated_at_...
 *
 * "test" is ignored as its value is empty, and "subtest_1"'s value is truncated.
 */
import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

const MAX_PARAMS = 8;

@customElement('graph-title-sk')
export class GraphTitleSk extends LitElement {
  @property({ attribute: false })
  titleEntries: Map<string, string> | null = null;

  @property({ type: Number })
  numTraces: number = 0;

  @state()
  private showShortTitle = true;

  createRenderRoot() {
    return this;
  }

  /**
   * Public function to set title entries and render.
   */
  set(titleEntries: Map<string, string> | null, numTraces: number): void {
    this.titleEntries = titleEntries;
    this.numTraces = numTraces;
  }

  /**
   * Generates the HTML for this.titleEntries. Empty keys or values
   * will result in the entry being ignored. Values longer than
   * 25 characters are truncated to avoid crowding.
   *
   * @returns - a list of HTML-formatted titleEntries.
   */
  private getTitleHtml(): TemplateResult[] {
    if (this.titleEntries === null || this.numTraces === 0) {
      return [];
    }

    if (this.titleEntries.size === 0 && this.numTraces > 0) {
      return [html`<h1>Multi-trace Graph (${this.numTraces} traces)</h1>`];
    }

    const elems: TemplateResult[] = [];

    const showShort = this.showShortTitle && this.titleEntries.size > MAX_PARAMS;

    let index = 0;
    this.titleEntries.forEach((value, key) => {
      if (showShort && index >= MAX_PARAMS) {
        return;
      }
      index++;

      if (value !== '' && key !== '') {
        const elem = html`
          <div class="column">
            <div class="param">${key}</div>
            <div class="hover-to-show-text" title=${value}>${value}</div>
          </div>
        `;
        elems.push(elem);
      }
    });

    if (showShort) {
      const elem = html`
        <md-text-button class="showMore" @click=${this.showFullTitle}>
          Show Full Title
        </md-text-button>
      `;
      elems.push(elem);
    }
    return elems;
  }

  showFullTitle() {
    this.showShortTitle = false;
  }

  showShortTitles() {
    this.showShortTitle = true;
  }

  render() {
    return html` <div id="container" ?hidden=${this.numTraces === 0}>${this.getTitleHtml()}</div> `;
  }
}
