{
  'targets': [
    {
      'configurations': {
        'Debug': { },
        'Release': { },
        'Release_Developer': { }
      },
      'cflags!': [
        '-Werror'
      ],
      'target_name': '{{.Hash}}',
      'type': 'executable',
      'dependencies': [
        'skia_lib.gyp:skia_lib',
        'gputest.gyp:skgputest',
        'pdf.gyp:pdf',
        'flags.gyp:flags',
        'tools.gyp:sk_tool_utils'
      ],
      'include_dirs': [
        '../include/config',
        '../include/core',
        '../include/gpu',
        '../tools/flags',
        '../src/core',
        '../src/gpu'
      ],
      'conditions': [
        ['skia_os == "mac"', {
                'defines': ['SK_UNSAFE_BUILD_DESKTOP_ONLY=1']
        }]
      ],
      'sources': [
        '../../cache/src/{{.Hash}}.cpp',
        '../../fiddle_main/main.cpp'
      ],
    },
    {
      'configurations': {
        'Debug': { },
        'Release': { },
        'Release_Developer': { }
      },
      'cflags!': [
        '-Werror'
      ],
      'target_name': 'fiddle_secwrap',
      'type': 'executable',
      'sources': [
        '../../fiddle_main/fiddle_secwrap.cpp'
      ],
    },
  ]
}
