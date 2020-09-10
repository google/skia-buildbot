// Force the demo page into darkmode.
import { DARKMODE_CLASS } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

window.localStorage.setItem(DARKMODE_CLASS, 'true');

import './index';
import { TextareaNumbersSk } from './textarea-numbers-sk';

const textarea = document.querySelector<TextareaNumbersSk>(
  'textarea-numbers-sk'
)!;

textarea.value = `  void draw(SkCanvas* canvas) {
  SkPaint p;
  p.setColor(SK_ColorRED);
  p.setAntiAlias(true);
  p.setStyle(SkPaint::kStroke_Style)
  p.setStrokeWidth(10);

  canvas->drawLine(20, 20, 100, 100, p
}`;
textarea.setErrorLine(4);
textarea.setErrorLine(7);

document
  .querySelector<HTMLButtonElement>('#clear_error_lines')!
  .addEventListener('click', () => {
    textarea.clearErrors();
  });
