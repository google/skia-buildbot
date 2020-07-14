import './index';
import { $$ } from 'common-sk/modules/dom';

(function() {
  $$('named-edit-sk').state = {
    name: 'Octopus_Generator_Animated',
    hash: 'ad161cfe21bb38bcec264bbacecbe93a',
    status: 'OK',
  };
  $$('named-edit-sk').show();

  function display(e) {
    $$('#events').textContent = `Event: ${e.type}\nDetail: ${JSON.stringify(e.detail, null, '  ')}`;
  }

  document.addEventListener('named-edit-complete', display);
}());
