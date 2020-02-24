import '../node_modules/@webcomponents/custom-elements/custom-elements.min';

import '../modules/body';
import '../modules/perf-scaffold-sk';
import '../modules/explore-sk';

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js');
  });
}
