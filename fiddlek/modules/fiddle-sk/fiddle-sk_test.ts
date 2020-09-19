import { assert } from 'chai';
import { FiddleSk } from './fiddle-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';


describe('fiddle-sk', () => {
  const newInstance = setUpElementUnderTest<FiddleSk>('fiddle-sk');

  let element: FiddleSk;
  beforeEach(() => {
    element = newInstance((fiddle: FiddleSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
      fiddle.config = {
        display_options: true,
        embedded: false,
        cpu_embedded: true,
        gpu_embedded: true,
        options_open: true,
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
        width: 128,
        height: 128,
        animated: false,
        duration: 5,
        offscreen: true,
        offscreen_width: 256,
        offscreen_height: 256,
        offscreen_sample_count: 1,
        offscreen_texturable: false,
        offscreen_mipmap: false,
        source: 1,
        source_mipmap: true,
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
    });
  });

  describe('some action', () => {
    it('some result', () => {
      assert.isNotNull((element));
    });
  });
});
