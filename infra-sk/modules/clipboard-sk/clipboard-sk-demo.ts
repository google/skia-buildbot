import { $$ } from 'common-sk/modules/dom';
import { ClipboardSk } from './clipboard-sk';
import './index';

// If a clipboard value is too expensive to calculate all the time, for example,
// a CSV file, then you can set the `calculatedValue` property on the
// clipboard-sk element and that will only be called if the user actually clicks
// on the element.
$$<ClipboardSk>('#onthefly')!.calculatedValue = async (): Promise<string> => 'This is the altered value.';
