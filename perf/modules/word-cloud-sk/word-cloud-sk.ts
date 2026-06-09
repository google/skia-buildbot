/**
 * @module modules/word-cloud-sk
 * @description <h2><code>word-cloud-sk</code></h2>
 *
 * Displays the key-value pairs found in a cluster, and also
 * shows the relative frequency of how often they appear.
 *
 * @example
 */
import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import { ValuePercent } from '../json';

@customElement('word-cloud-sk')
export class WordCloudSk extends LitElement {
  /** A serialized slice of ValuePercent.

      For example:

      [
        {value:"config=565", percent: 60},
        {value:"config=8888", percent: 40},
        {value:"cpu_or_gpu=cpu", percent: 20},
        {value:"cpu_or_gpu=gpu", percent: 10},
      ]
  */
  @property({ type: Array })
  items: ValuePercent[] = [];

  protected createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <table>
        ${repeat(
          this.items,
          (item) => item.value,
          (item) => html`
            <tr>
              <td class="value">${item.value}</td>
              <td class="textpercent">${item.percent}%</td>
              <td class="percent" title="${item.percent}%">
                <div style="width: ${item.percent}px"></div>
              </td>
            </tr>
          `
        )}
      </table>
    `;
  }
}
