import './index';
import { FiddleSk } from './fiddle-sk';
import './fiddle-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { ThemeChooserSk } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

const fiddle = document.querySelector<FiddleSk>('fiddle-sk')!;

// We need to wait for images or video elements to load before taking
// screenshots for Gold, but puppeteer can't listen for events, so we stuff a
// 'pre' element in #mode_complete after switching to each mode. We also need to
// remove that 'pre' element as we start switching to each mode.
//
// This also makes it easy to confirm that the buttons work when viewing the
// demo page.
const beforeMode = () => {
  document.querySelector('#mode_complete')!.innerHTML = '';
  fiddle.runResults = {
    compile_errors: [],
    runtime_error: '',
    fiddleHash: '',
    text: '',
  };
};

const modeComplete = () => {
  document.querySelector('#mode_complete')!.innerHTML = '<pre>Done.</pre>';
};

document.querySelector('#mode_start')!.addEventListener('click', () => {
  beforeMode();
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
  modeComplete();
});

document
  .querySelector('#mode_after_run_errors')!.addEventListener('click', () => {
    beforeMode();
    fiddle.config = {
      display_options: true,
      embedded: false,
      cpu_embedded: true,
      gpu_embedded: true,
      options_open: false,
      basic_mode: false,
      domain: 'https://fiddle.skia.org',
      bug_link: false,
      sources: [1, 2, 3],
      loop: true,
      play: true,
    };

    fiddle.runResults = {
      compile_errors: [
        {
          text: 'FAILED: obj/tools/fiddle/fiddle.draw.o ',
          line: 0,
          col: 0,
        },
        {
          text:
            'clang++ -MD -MF obj/tools/fiddle/fiddle.draw.o.d -DNDEBUG -DSK_GL -DSK_SUPPORT_PDF -DSK_CODEC_DECODES_JPEG -DSK_ENCODE_JPEG -DSK_ENABLE_ANDROID_UTILS -DSK_USE_LIBGIFCODEC -DSK_HAS_HEIF_LIBRARY -DSK_CODEC_DECODES_PNG -DSK_ENCODE_PNG -DSK_CODEC_DECODES_RAW -DSK_ENABLE_SKSL_INTERPRETER -DSKVM_JIT_WHEN_POSSIBLE -DSK_CODEC_DECODES_WEBP -DSK_ENCODE_WEBP -DSK_XML -DSK_GAMMA_APPLY_TO_A8 -DSK_ALLOW_STATIC_GLOBAL_INITIALIZERS=1 -DGR_TEST_UTILS=1 -DSK_R32_SHIFT=16 -DSK_ENABLE_SKOTTIE -DSK_SHAPER_HARFBUZZ_AVAILABLE -DSK_UNICODE_AVAILABLE -I../.. -I../../third_party/externals/libgifcodec -I../../include/third_party/vulkan -I../.. -Igen -I../../modules/skottie/include -I../../modules/skshaper/include -Wno-attributes -fstrict-aliasing -fPIC -fvisibility=hidden -O3 -fdata-sections -ffunction-sections -g -Wall -Wextra -Winit-self -Wpointer-arith -Wsign-compare -Wvla -Wno-deprecated-declarations -Wno-maybe-uninitialized -Wno-psabi -fcolor-diagnostics -Weverything -Wno-unknown-warning-option -Wno-nonportable-include-path -Wno-nonportable-system-include-path -Wno-cast-align -Wno-cast-qual -Wno-conversion -Wno-disabled-macro-expansion -Wno-documentation -Wno-documentation-unknown-command -Wno-double-promotion -Wno-exit-time-destructors -Wno-float-equal -Wno-format-nonliteral -Wno-global-constructors -Wno-missing-prototypes -Wno-missing-variable-declarations -Wno-pedantic -Wno-reserved-id-macro -Wno-shadow -Wno-shift-sign-overflow -Wno-signed-enum-bitfield -Wno-switch-enum -Wno-undef -Wno-unreachable-code -Wno-unreachable-code-break -Wno-unreachable-code-return -Wno-unused-macros -Wno-unused-member-function -Wno-unused-template -Wno-zero-as-null-pointer-constant -Wno-thread-safety-negative -Wno-non-c-typedef-for-linkage -Wsign-conversion -Wno-covered-switch-default -Wno-deprecated -Wno-missing-noreturn -Wno-old-style-cast -Wno-padded -Wno-newline-eof -Wdeprecated-anon-enum-enum-conversion -Wdeprecated-array-compare -Wdeprecated-attributes -Wdeprecated-comma-subscript -Wdeprecated-copy -Wdeprecated-dynamic-exception-spec -Wdeprecated-enum-compare -Wdeprecated-enum-compare-conditional -Wdeprecated-enum-enum-conversion -Wdeprecated-enum-float-conversion -Wdeprecated-increment-bool -Wdeprecated-register -Wdeprecated-this-capture -Wdeprecated-volatile -Wdeprecated-writable-str -Wno-sign-conversion -Wno-unused-parameter -I/tmp/swiftshader/include -DGR_EGL_TRY_GLES3_THEN_GLES2 -g0 -std=c++17 -fvisibility-inlines-hidden -fno-exceptions -fno-rtti -Wnon-virtual-dtor -Wno-noexcept-type -Wno-redundant-move -Wno-abstract-vbase-init -Wno-weak-vtables -Wno-c++98-compat -Wno-c++98-compat-pedantic -Wno-undefined-func-template -c ../../tools/fiddle/draw.cpp -o obj/tools/fiddle/fiddle.draw.o',
          line: 0,
          col: 0,
        },
        {
          text: "draw.cpp:5:39: error: expected ';' after expression",
          line: 5,
          col: 39,
        },
        {
          text: '    p.setStyle(SkPaint::kStroke_Style)',
          line: 0,
          col: 0,
        },
        {
          text: '                                      ^',
          line: 0,
          col: 0,
        },
        {
          text: '                                      ;',
          line: 0,
          col: 0,
        },
        {
          text: "draw.cpp:6:25: error: expected ';' after expression",
          line: 6,
          col: 25,
        },
        {
          text: '    p.setStrokeWidth(10)',
          line: 0,
          col: 0,
        },
        {
          text: '                        ^',
          line: 0,
          col: 0,
        },
        {
          text: '                        ;',
          line: 0,
          col: 0,
        },
        {
          text: '2 errors generated.',
          line: 0,
          col: 0,
        },
      ],
      runtime_error: '',
      fiddleHash: '44dfa2e23262b5353e769b8040b48f19',
      text: `void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style)
    p.setStrokeWidth(10)

    canvas->drawLine(20, 20, 100, 100, p);
}`,
    };
    modeComplete();
  });

document
  .querySelector('#mode_after_run_success')!.addEventListener('click', () => {
    beforeMode();

    fiddle.config = {
      display_options: true,
      embedded: false,
      cpu_embedded: true,
      gpu_embedded: true,
      options_open: false,
      basic_mode: false,
      domain: 'https://fiddle.skia.org',
      bug_link: false,
      sources: [1, 2, 3],
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
      fiddleHash: '5395a831409c92c22262fe20f3ecdca8',
      text: `
void draw(SkCanvas* canvas) {
  SkPaint p;
  p.setColor(SK_ColorRED);
  p.setAntiAlias(true);
  p.setStyle(SkPaint::kStroke_Style);
  p.setStrokeWidth(10);
  canvas->drawLine(20, 20, 100, 100, p);
}`,
    };

    // Wait for all the result images to load before signalling we are done.
    const promises : Promise<never>[] = [];
    fiddle.querySelectorAll('img.result_image').forEach((element) => {
      promises.push(new Promise<never>((resolve) => {
        element.addEventListener('load', () => {
          resolve();
        });
      }));
    });
    Promise.all(promises).then(modeComplete);
  });

document.querySelector('#mode_animation')!.addEventListener('click', () => {
  beforeMode();

  fiddle.config = {
    display_options: true,
    embedded: false,
    cpu_embedded: true,
    gpu_embedded: true,
    options_open: false,
    basic_mode: false,
    domain: 'https://fiddle.skia.org',
    bug_link: false,
    sources: [1, 2, 3],
    loop: false,
    play: false,
  };

  fiddle.options = {
    textOnly: false,
    srgb: false,
    f16: false,
    width: 256,
    height: 256,
    animated: true,
    duration: 1,
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
    fiddleHash: 'd7837be52c71542af0f80cd61aab421f',
    text: `
void draw(SkCanvas* canvas) {
  SkPaint p;
  p.setColor(SK_ColorRED);
  p.setAntiAlias(true);
  p.setStyle(SkPaint::kStroke_Style);
  p.setStrokeWidth(10);

  canvas->drawLine(20+100*frame, 20, 100, 100, p);
}
    `,
  };

  // Wait for all the result videos to load before signalling we are done.
  const promises : Promise<never>[] = [];
  fiddle.querySelectorAll('video').forEach((element) => {
    promises.push(new Promise<never>((resolve) => {
      element.addEventListener('canplay', () => {
        resolve();
      });
    }));
  });
  Promise.all(promises).then(modeComplete);
});

document.querySelector('#mode_basic')!.addEventListener('click', () => {
  beforeMode();

  fiddle.config = {
    display_options: true,
    embedded: true,
    cpu_embedded: true,
    gpu_embedded: true,
    options_open: true,
    basic_mode: true,
    domain: 'https://fiddle.skia.org',
    bug_link: false,
    sources: [1, 2, 3],
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
    duration: 5,
    offscreen: false,
    offscreen_width: 256,
    offscreen_height: 256,
    offscreen_sample_count: 1,
    offscreen_texturable: false,
    offscreen_mipmap: false,
    source: 0,
    source_mipmap: false,
  };

  fiddle.runResults = {
    compile_errors: [],
    runtime_error: '',
    fiddleHash: '5395a831409c92c22262fe20f3ecdca8',
    text: `void draw(SkCanvas* canvas) {
  SkPaint p;
  p.setColor(SK_ColorRED);
  p.setAntiAlias(true);
  p.setStyle(SkPaint::kStroke_Style);
  p.setStrokeWidth(10);
  canvas->drawLine(20, 20, 100, 100, p);
}`,
  };

  fiddle.querySelector('img.cpu')!.addEventListener('load', modeComplete);
});

document.querySelector<HTMLButtonElement>('#mode_start')!.click();
document.querySelector<ThemeChooserSk>('theme-chooser-sk')!.darkmode = true;
