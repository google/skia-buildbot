import './newindex.scss';
import '../modules/themes/themes.scss';
import '../modules/fiddle-sk';
import { FiddleSk } from '../modules/fiddle-sk/fiddle-sk';
import { FiddleContext } from '../modules/json';
import '../../infra-sk/modules/theme-chooser-sk';

const fiddle = document.querySelector<FiddleSk>('fiddle-sk')!;
fiddle.config = {
  display_options: true,
  embedded: false,
  cpu_embedded: false,
  gpu_embedded: false,
  options_open: false,
  basic_mode: false,
  domain: 'https://fiddle.skia.org',
  bug_link: true,
  sources: [1, 2, 3, 4, 5, 6],
  loop: true,
  play: true,
};

fiddle.options = {
  textOnly: false,
  srgb: false,
  f16: false,
  width: 256,
  height: 256,
  animated: false,
  duration: 2,
  offscreen: false,
  offscreen_width: 256,
  offscreen_height: 256,
  offscreen_sample_count: 1,
  offscreen_texturable: false,
  offscreen_mipmap: false,
  source: 1,
  source_mipmap: false,
};

fiddle.runResults = {
  compile_errors: [],
  runtime_error: '',
  fiddleHash: '',
  text: `void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style);
    p.setStrokeWidth(10);

    canvas->drawLine(20, 20, 100, 100, p);
}`,
};

declare global {
  interface Window {
    sk: {
        fiddle: FiddleContext;
    };
  }
}

// If the context is supplied, i.e. we are being served from /c/<name>, then use
// that context in the element.
if (window.sk.fiddle.fiddlehash) {
  fiddle.context = window.sk.fiddle;
}
