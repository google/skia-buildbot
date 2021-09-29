import './index';
import { $$ } from 'common-sk/modules/dom';
import { testOnlySetSettings } from '../settings';
import { GoldScaffoldSk } from './gold-scaffold-sk';

testOnlySetSettings({
  title: 'Skia Public',
});

// Remove from DOM and reattach to trigger a re-render. This ensures that the GoldScaffoldSk
// instance will pick up the title from the above settings.
const goldScaffoldSk = $$<GoldScaffoldSk>('gold-scaffold-sk')!;
const parent = goldScaffoldSk.parentNode!;
parent.removeChild(goldScaffoldSk);
parent.appendChild(goldScaffoldSk);
