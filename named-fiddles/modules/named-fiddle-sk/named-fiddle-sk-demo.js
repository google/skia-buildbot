import './index';
import { $$ } from 'common-sk/modules/dom';

(function() {
  $$('#ok').state = {
    name: 'Octopus_Generator',
    hash: 'ad161cfe21bb38bcec264bbacecbe93a',
    status: 'OK',
    user: 'fred@example.com',
  };
  $$('#failed').state = {
    name: 'Octopus_Generator_Animated',
    hash: 'f8c5a15bb959b455eb98d680f0a848c8',
    status: 'Failed',
    user: 'barney@example.org',
  };

  function display(e) {
    $$('#events').textContent = `Event: ${e.type}\nDetail: ${JSON.stringify(e.detail, null, '  ')}`;
  }

  document.addEventListener('named-edit', display);
  document.addEventListener('named-delete', display);
}());
