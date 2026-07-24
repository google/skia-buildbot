import { html, TemplateResult } from 'lit/html.js';

export interface TaskSummary {
  analysis?: string;
  errorMessage?: string;
}

/**
 * Returns a template displaying the task summary as table rows.
 * Designed to be rendered directly inside a <table> or <tbody> element.
 */
export function taskSummaryRows(s: TaskSummary | null): TemplateResult {
  if (!s || (!s.errorMessage && !s.analysis)) {
    return html``;
  }
  return html`
    ${s.errorMessage
      ? html`
          <tr>
            <td><b>Error Message:</b></td>
            <td class="pre"><code>${s.errorMessage}</code></td>
          </tr>
        `
      : html``}
    ${s.analysis
      ? html`
          <tr>
            <td>Analysis:</td>
            <td>${s.analysis}</td>
          </tr>
        `
      : html``}
  `;
}
