import './index';
import { $$ } from 'common-sk/modules/dom';

(function() {
  function populate() {
    $$('named-fiddles-sk')._named_fiddles = [
      {
        name: 'Octopus_Generator',
        hash: 'ad161cfe21bb38bcec264bbacecbe93a',
        status: 'OK',
      },
      {
        name: 'Octopus_Generator_Animated',
        hash: 'f8c5a15bb959b455eb98d680f0a848c8',
        status: 'Failed',
      },
    ];
    $$('named-fiddles-sk')._render();
  }
  setTimeout(populate, 1000);
}());
