import { TemplateResult } from 'lit/html.js';
import { isOneOfDomains } from './domains';

export function renderByDomain(
  template: TemplateResult,
  domains: string[]
): TemplateResult | null {
  return isOneOfDomains(domains) ? template : null;
}
