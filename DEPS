use_relative_paths = True

vars = {
  'cpython_version':    'version:2@2.7.18.chromium.47',
  'cpython3_version':   'version:3@3.11.9.chromium.36',
  'luci_tools_version': 'git_revision:aaf80ec8202c91ad9c7aa20ee3841d00d4a7d51c',
  'skia_tools_version': 'git_revision:00b2af652822bf187ec5b27e1979f126b255f9cd',
  'tools_git_version':  'version:3@2.51.0.chromium.11',
}

deps = {
  'depot_tools': {
    'url': 'https://chromium.googlesource.com/chromium/tools/depot_tools.git@838e4c6aa7b68d7a4de1c2f2cd516104d729eff3',
    'condition': 'False',
  },
  'cipd_bin_packages': {
    'packages': [
      {
        'package': 'infra/3pp/tools/git/linux-amd64',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/3pp/tools/git/linux-arm64',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/3pp/tools/git/linux-armv6l',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/3pp/tools/git/mac-amd64',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/3pp/tools/git/windows-386',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/3pp/tools/git/windows-amd64',
        'version': Var('tools_git_version'),
      },
      {
        'package': 'infra/tools/git/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci-auth/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/docker-credential-luci/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/git-credential-luci/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/isolate/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/lucicfg/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/swarming/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/vpython3/${{platform}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'skia/bots/gsutil',
        'version': 'version:6',
      },
      {
        'package': 'skia/bots/patch_linux_amd64',
        'version': 'version:0',
      },
      {
        'package': 'skia/tools/goldctl/${{platform}}',
        'version': Var('luci_tools_version'),
      },
    ],
    'dep_type': 'cipd',
    'condition': 'False',
  },
  'cipd_bin_packages/cpython': {
    'packages': [
      {
        'package': 'infra/3pp/tools/cpython/linux-amd64',
        'version': Var('cpython_version'),
      },
      {
        'package': 'infra/3pp/tools/cpython/linux-arm64',
        'version': Var('cpython_version'),
      },
      {
        'package': 'infra/3pp/tools/cpython/linux-armv6l',
        'version': Var('cpython_version'),
      },
      {
        'package': 'infra/3pp/tools/cpython/mac-amd64',
        'version': Var('cpython_version'),
      },
      {
        'package': 'infra/3pp/tools/cpython/windows-386',
        'version': Var('cpython_version'),
      },
      {
        'package': 'infra/3pp/tools/cpython/windows-amd64',
        'version': Var('cpython_version'),
      },
    ],
    'dep_type': 'cipd',
    'condition': 'False',
  },
  'cipd_bin_packages/cpython3': {
    'packages': [
      {
        'package': 'infra/3pp/tools/cpython3/linux-amd64',
        'version': Var('cpython3_version')
      },
      {
        'package': 'infra/3pp/tools/cpython3/linux-arm64',
        'version': Var('cpython3_version')
      },
      {
        'package': 'infra/3pp/tools/cpython3/linux-armv6l',
        'version': Var('cpython3_version')
      },
      {
        'package': 'infra/3pp/tools/cpython3/mac-amd64',
        'version': Var('cpython3_version')
      },
      {
        'package': 'infra/3pp/tools/cpython3/windows-386',
        'version': Var('cpython3_version')
      },
      {
        'package': 'infra/3pp/tools/cpython3/windows-amd64',
        'version': Var('cpython3_version')
      },
    ],
    'dep_type': 'cipd',
    'condition': 'False',
  },
  'task_drivers': {
    'packages': [
      {
        'package': 'skia/tools/bazel_build_all/${{platform}}',
        'version': Var('skia_tools_version'),
      },
      {
        'package': 'skia/tools/bazel_test_all/${{platform}}',
        'version': Var('skia_tools_version'),
      },
      {
        'package': 'skia/tools/command_wrapper/${{platform}}',
        'version': Var('skia_tools_version'),
      },
      {
        'package': 'skia/tools/presubmit/${{platform}}',
        'version': Var('skia_tools_version'),
      },
    ],
    'dep_type': 'cipd',
    'condition': 'False',
  },
  '': {
    'packages': [
      {
        'package': 'infra/tools/cipd/${{os}}-${{arch}}',
        'version': Var('luci_tools_version'),
      },
      {
        'package': 'infra/tools/luci/kitchen/${{platform}}',
        'version': Var('luci_tools_version'),
      },
    ],
    'dep_type': 'cipd',
    'condition': 'False',
  },
}

recursedeps = []
