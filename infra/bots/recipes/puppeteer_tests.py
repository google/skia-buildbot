DEPS = [
    'recipe_engine/context',
    'recipe_engine/path',
    'recipe_engine/step',
]


def RunSteps(api):
  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  golden_dir = api.path['start_dir'].join('buildbot').join('golden')

  with api.context(cwd=golden_dir, env={'DOCKER_CONFIG': '/home/chrome-bot/.docker'}):
    api.step('run puppeteer tests', cmd=['make', 'puppeteer-test-upload-digests'])


def GenTests(api):
  yield (
      api.test('Infra-PerCommit-Puppeteer')
  )
