PYTHON_VERSION_COMPATIBILITY = "PY3"

DEPS = [
    'recipe_engine/context',
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/step',
]


def RunSteps(api):
  revision = api.properties['revision']

  # Are we in a trybot? If so, retrieve issue, patchset, etc.
  is_trybot = False
  issue = None
  patch_set = None
  buildbucket_build_id = None
  # pylint: disable=too-many-boolean-expressions
  if (api.properties.get('patch_issue', '0') != '0' and
      api.properties.get('patch_set', '0') != '0' and
      api.properties.get('revision', '0') != '0' and
      api.properties.get('buildbucket_build_id', '0') != '0'):
    is_trybot = True
    issue = api.properties['patch_issue']
    patch_set = api.properties['patch_set']
    buildbucket_build_id = api.properties['buildbucket_build_id']

  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  # Run Puppeteer tests inside a Docker container.
  buildbot_dir = api.path['start_dir'].join('buildbot')
  with api.context(cwd=buildbot_dir,
                   env={'DOCKER_CONFIG': '/home/chrome-bot/.docker'}):
    api.step('run puppeteer tests', cmd=['make', 'puppeteer-tests'])

  # Upload any digests produced by Puppeteer tests to Gold.
  with api.context(cwd=buildbot_dir.join('puppeteer-tests')):
    upload_digests_cmd = [
        'python3',
        'upload-screenshots-to-gold.py',
        '--images_dir', './output',
        # This is [START_DIR]/cipd_bin_packages/goldctl.
        '--path_to_goldctl', '../../cipd_bin_packages/goldctl',
        '--revision', revision,
    ]
    if is_trybot:
      api.step('upload digests (tryjob)',
               upload_digests_cmd + ['--issue', issue,
                                     '--patch_set', patch_set,
                                     '--task_id', buildbucket_build_id])
    else:
      api.step('upload digests (non-tryjob)', upload_digests_cmd)


def GenTests(api):
  yield (
      api.test('Infra-PerCommit-Puppeteer') +
      api.properties(revision='78e0b810cc3adc002a09c5190bb104afdcbbe3e1')
  )

  yield (
      api.test('Infra-PerCommit-Puppeteer_tryjob') +
      api.properties(patch_issue='123456',
                     patch_set='3',
                     revision='78e0b810cc3adc002a09c5190bb104afdcbbe3e1',
                     buildbucket_build_id='8894409419339087024')
  )

