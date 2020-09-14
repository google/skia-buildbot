import './index';
import { FiddleSk } from './fiddle-sk';

const fiddle = document.querySelector<FiddleSk>('fiddle-sk')!;

document.querySelector('#mode_start')?.addEventListener('click', () => {
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

document.querySelector('#mode_after_run')?.addEventListener('click', () => {
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
});
