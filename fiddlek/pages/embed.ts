import '../modules/fiddle-embed';
import { FiddleSkFiddleSuccessEventDetail } from '../modules/fiddle-sk/fiddle-sk';

document.querySelector('fiddle-sk')!.addEventListener('fiddle-success', (e) => {
  const name = (e as CustomEvent<FiddleSkFiddleSuccessEventDetail>).detail;
  window.history.pushState(null, '', `/c/${name}`);
});
