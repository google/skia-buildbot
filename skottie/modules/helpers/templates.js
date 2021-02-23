import { isOneOfDomains } from './domains';

const renderByDomain = (template, domains) => (isOneOfDomains(domains) ? template : null);

export {
  renderByDomain,
};
