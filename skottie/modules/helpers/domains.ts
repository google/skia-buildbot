// Helper to identify domain and enable or disable features based on them

const currentDomain = window.location.hostname;

const isDomain = (domain: string): boolean => domain === currentDomain;

const isOneOfDomains = (domains: string[]): boolean =>
  domains.includes(currentDomain);

export { isDomain, isOneOfDomains };
