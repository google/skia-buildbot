/**
 * @module modules/query-count-sk
 * @description <h2><code>query-count-sk</code></h2>
 *
 * Reports the number of matches for a given query.
 *
 * @attr {string} current_query - The current query to count against.
 *
 * @attr {string} url - The URL to POST the query to.
 *
 * @evt paramset-changed - An event with the updated paramset in e.detail
 *   from the fetch response.
 *
 */
import { html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Task, TaskStatus } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { CountHandlerRequest, CountHandlerResponse, ReadOnlyParamSet } from '../json';
import '../../../elements-sk/modules/spinner-sk';

@customElement('query-count-sk')
export class QueryCountSk extends LitElement {
  @property({ type: String })
  url: string = '';

  @property({ type: String, attribute: 'current_query' })
  current_query: string = '';

  private _fetchTask = new Task(this, {
    task: async (
      [url, current_query]: readonly [string, string],
      { signal }: { signal: AbortSignal }
    ) => {
      if (!url || !current_query) {
        return 0;
      }

      const now = Math.floor(Date.now() / 1000);
      const body: CountHandlerRequest = {
        q: current_query,
        end: now,
        begin: now - 24 * 60 * 60,
      };

      try {
        const response = await fetch(url, {
          method: 'POST',
          signal,
          body: JSON.stringify(body),
          headers: {
            'Content-Type': 'application/json',
          },
        });
        const json = (await jsonOrThrow(response)) as CountHandlerResponse;
        this.dispatchEvent(
          new CustomEvent<ReadOnlyParamSet>('paramset-changed', {
            detail: json.paramset,
            bubbles: true,
            composed: true,
          })
        );
        return json.count;
      } catch (msg: any) {
        if (msg.name === 'AbortError') {
          throw msg;
        }
        errorMessage(msg);
        throw msg;
      }
    },
    args: () => [this.url, this.current_query] as const,
  });

  createRenderRoot() {
    return this;
  }

  render() {
    // Reset count to 0 while loading to match legacy behavior.
    const isLoading =
      this._fetchTask.status === TaskStatus.PENDING ||
      this._fetchTask.status === TaskStatus.INITIAL;
    const count = isLoading ? 0 : (this._fetchTask.value ?? 0);

    return html`
      <div>
        <span>${count}</span>
        <spinner-sk ?active=${isLoading}></spinner-sk>
      </div>
    `;
  }
}
