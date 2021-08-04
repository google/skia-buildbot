// Helper to identify domain and enable or disable features based on them

const currentDomain = window.location.hostname;

const supportedDomains = {
  SKOTTIE_INTERNAL: 'skottie-internal.skia.org',
  SKOTTIE_TENOR: 'skottie-tenor.skia.org',
  SKOTTIE: 'skottie.skia.org',
  LOCALHOST: 'localhost',
};

const isDomain = (domain: string): boolean => domain === currentDomain;

const isOneOfDomains = (domains: string): boolean => domains.includes(currentDomain);

export {
  isDomain,
  isOneOfDomains,
  supportedDomains,
};
