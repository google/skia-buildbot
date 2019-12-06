DEPS = [
    'recipe_engine/context',
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/step',
]


def RunSteps(api):
  # Retrieve issue and patchset if we're in a trybot.
  is_trybot = False
  issue = None
  patchset = None
  if (api.properties.get('patch_issue', '') and
      api.properties['patch_issue'] != '0' and
      api.properties.get('patch_set', '') and
      api.properties['patch_set'] != '0' and
      api.properties.get('patch_ref', '')):
    is_trybot = True
    issue = api.properties['patch_issue']
    patchset = api.properties['patch_set']

  def printApiProperties(property):
    api.step(
        'echo api.properties["%s"]' % property,
        cmd=['echo', ('api.properties["%s"] = ' % property) + api.properties.get(property, 'None')])
  printApiProperties('patch_issue')
  printApiProperties('patch_set')
  printApiProperties('patch_ref')

  api.step('echo is_trybot', cmd=['echo', 'is_trybot = ' + str(is_trybot)])
  api.step('echo issue', cmd=['echo', 'issue = ' + str(issue)])
  api.step('echo patchset', cmd=['echo', 'patchset = ' + str(patchset)])

  api.step('gsutil ls gs://skia-gold-skia-infra', cmd=['gsutil', 'ls', 'gs://skia-gold-skia-infra'])

  api.step('about to sleep for 10 minutes', cmd=['echo'])
  api.step('sleep for 10 minutes', cmd=['sleep', '600'])

  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  golden_dir = api.path['start_dir'].join('buildbot').join('golden')
  with api.context(cwd=golden_dir, env={'DOCKER_CONFIG': '/home/chrome-bot/.docker'}):
    api.step('run puppeteer tests', cmd=['make', 'puppeteer-test'])

  upload_digests_cmd = ['python', 'upload-screenshots-to-gold.py']
  puppeteer_tests_dir = golden_dir.join('puppeteer-tests')
  with api.context(cwd=puppeteer_tests_dir):
    if is_trybot:
      cmd = upload_digests_cmd + ['--issue', issue, '--patch_set', patchset]
      api.step('upload digests (with issue and patch_set)', cmd)
    else:
      api.step('upload digests (no issue nor patch_set)', upload_digests_cmd)

def GenTests(api):
  yield (
      api.test('Infra-PerCommit-Puppeteer')
  )

  yield (
      api.test('Infra-PerCommit-Puppeteer_tryjob') +
      api.properties(patch_issue='123456',
                     patch_set='3',
                     patch_ref='refs/changes/95/123456/3')
  )
