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
      'target_name': 'fiddle_secwrap',
      'type': 'executable',
      'sources': [
        '../../fiddle_main/fiddle_secwrap.cpp'
      ],
    },
  ]
}
