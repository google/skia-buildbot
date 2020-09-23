import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

$$<ThemeChooserSk>('theme-chooser-sk')!.darkmode = true;

$$('#mode_start')!.addEventListener('click', () => {
  $$('#complete')!.innerHTML = '';

  // The number of images that have emitted 'load' events.
  let loadedCount = 0;
  $('fiddle-embed').forEach((ele) => {
    // Set the fiddle to load for each fiddle-embed element.
    ele.setAttribute('name', '5e3a25b5d5dfb2195a3c5fd5959245a5');

    // Once that context has been loaded we add a listener for each image to load.
    ele.addEventListener('context-loaded', () => {
      $('img', ele).forEach((img) => {
        img.addEventListener('load', () => {
          loadedCount++;

          // Once all six images have loaded we can declare we are done. Note we
          // have to count to 6 since there are 3 controls, and even if the
          // image isn't displayed, i.e. display: none; it will still generate a
          // 'load' event.
          if (loadedCount === 6) {
            $$('#complete')!.innerHTML = '<pre>Done.</pre>';
          }
        });
      });
    });
  });
});
