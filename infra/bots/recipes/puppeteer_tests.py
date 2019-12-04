DEPS = [
    'recipe_engine/context',
    'recipe_engine/path',
    'recipe_engine/step',
]


def RunSteps(api):
  api.step('say hello', cmd=['echo', 'hello'])
  api.step('print cwd', cmd=['pwd'])
  api.step('ls', cmd=['ls', '-lh'])

  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  infra_dir = api.path['start_dir'].join('buildbot')
  golden_dir = infra_dir.join('golden')

  with api.context(cwd=infra_dir):
    api.step('ls from infra dir', cmd=['ls', '-lh'])

  with api.context(cwd=golden_dir):
    api.step('ls from golden dir', cmd=['ls', '-lh'])

    api.step('sleep for 5 minutes so I can ssh into the bot', cmd=['sleep', '300'])

    api.step('run puppeteer tests', cmd=['make', 'puppeteer-test'])

  api.step('say bye', cmd=['echo', 'all', 'done,', 'exiting'])


def GenTests(api):
  yield (
      api.test('Hello-World')
  )
