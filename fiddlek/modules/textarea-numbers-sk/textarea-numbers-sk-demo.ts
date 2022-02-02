// Force the demo page into darkmode.
import { DARKMODE_CLASS } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

import './index';
import { TextareaNumbersSk } from './textarea-numbers-sk';

window.localStorage.setItem(DARKMODE_CLASS, 'true');

const textarea = document.querySelector<TextareaNumbersSk>('textarea-numbers-sk')!;

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

document
  .querySelector<HTMLButtonElement>('#add_fold_tokens')!
  .addEventListener('click', () => {
    textarea.value = `  void draw(SkCanvas* canvas) {
      // Setup code // SK_FOLD_START
      SkPaint p;
      p.setColor(SK_ColorRED);
      p.setAntiAlias(true);
      p.setStyle(SkPaint::kStroke_Style)
      p.setStrokeWidth(10);
      // SK_FOLD_END

      canvas->drawLine(20, 20, 100, 100, p
    }`;
  });

document
  .querySelector<HTMLButtonElement>('#expand_fold')!
  .addEventListener('click', () => {
    // Set cursor on the foldmarker to expand the fold.
    textarea.setCursor(2, 22);
  });
