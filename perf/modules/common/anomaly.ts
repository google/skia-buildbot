/**
 * @module modules/common/anomaly
 * @description Anomaly utility functions.
 *
 */
import { html, TemplateResult } from 'lit';

export const formatNumber = (num: number): string =>
  num.toLocaleString('en-US', {
    maximumFractionDigits: 4,
  });

export const formatPercentage = (num: number): string =>
  num.toLocaleString('en-US', {
    maximumFractionDigits: 4,
    signDisplay: 'exceptZero',
  });

export const getPercentChange = (median_before: number, median_after: number): number => {
  const difference = median_after - median_before;
  // Division by zero is represented by infinity symbol.
  return (100 * difference) / median_before;
};

export const formatBug = (bugHostUrl: string, bugId: number): TemplateResult => {
  if (bugId === 0) {
    return html``;
  }
  if (bugId === -1) {
    return html`Invalid Alert`;
  }
  if (bugId === -2) {
    return html`Ignored Alert`;
  }
  // Trim the trailing '/' since we are adding it in the format.
  if (bugHostUrl.endsWith('/')) {
    bugHostUrl = bugHostUrl.substring(0, bugHostUrl.length - 1);
  }
  return html`<a href="${`${bugHostUrl}/${bugId}`}" target="_blank">${bugId}</a>`;
};
