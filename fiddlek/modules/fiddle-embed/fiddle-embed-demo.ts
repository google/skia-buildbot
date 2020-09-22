import './index';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

document.querySelector<ThemeChooserSk>('theme-chooser-sk')!.darkmode = true;


// Wait for all the result images to load before signalling we are done.
const promises: Promise<never>[] = [];
document.querySelectorAll('img.result_image').forEach((element) => {
  promises.push(new Promise<never>((resolve) => {
    element.addEventListener('load', () => {
      resolve();
    });
  }));
});
Promise.all(promises).then(() => {
  document.querySelector('#complete')!.innerHTML = '<pre>Done.</pre>';
});

document.querySelectorAll('fiddle-embed').forEach((ele) => {
  ele.setAttribute('name', '5e3a25b5d5dfb2195a3c5fd5959245a5');
});
