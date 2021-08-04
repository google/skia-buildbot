import { TemplateResult } from 'lit-html';
import { isOneOfDomains } from './domains';

const renderByDomain = (template: TemplateResult, domains: string): TemplateResult | null => (isOneOfDomains(domains) ? template : null);

export {
  renderByDomain,
};
