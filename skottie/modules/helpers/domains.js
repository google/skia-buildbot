// Helper to identify domain and enable or disable features based on them

const currentDomaint = window.location.hostname;

const supportedDomains = {
  SKOTTIE_INTERNAL: 'skottie-internal.skia.org',
  SKOTTIE_TENOR: 'skottie-tenor.skia.org',
  SKOTTIE: 'skottie.skia.org',
  LOCALHOST: 'localhost',
};

const isDomain = (domain) => domain === currentDomaint;

export {
  isDomain,
  supportedDomains,
};
