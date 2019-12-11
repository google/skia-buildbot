DEPS = [
    'recipe_engine/context',
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/step',
]


def RunSteps(api):
  revision = api.properties['revision']

  # Retrieve issue, patchset, etc. if we're in a trybot.
  is_trybot = False
  issue = None
  patch_set = None
  buildbucket_build_id = None
  if (api.properties.get('patch_issue', '') and
      api.properties['patch_issue'] != '0' and
      api.properties.get('patch_set', '') and
      api.properties['patch_set'] != '0' and
      api.properties.get('revision', '') and
      api.properties['revision'] != '0' and
      api.properties.get('buildbucket_build_id', '') and
      api.properties['buildbucket_build_id'] != '0'):
    is_trybot = True
    issue = api.properties['patch_issue']
    patch_set = api.properties['patch_set']
    buildbucket_build_id = api.properties['buildbucket_build_id']


  # Debugging stuff - delete before landing.
  def printApiProperties(property):
    api.step(
        'echo api.properties["%s"]' % property,
        cmd=['echo', ('api.properties["%s"] = ' % property) + api.properties.get(property, 'None')])
  printApiProperties('patch_issue')
  printApiProperties('patch_set')
  printApiProperties('patch_ref')
  printApiProperties('revision')
  printApiProperties('buildbucket_build_id')
  api.step('echo is_trybot', cmd=['echo', 'is_trybot = ' + str(is_trybot)])
  api.step('echo issue', cmd=['echo', 'issue = ' + str(issue)])
  api.step('echo patch_set', cmd=['echo', 'patch_set = ' + str(patch_set)])
  api.step('echo revision', cmd=['echo', 'revision = ' + str(revision)])
  api.step('echo buildbucket_build_id', cmd=['echo', 'buildbucket_build_id = ' + str(buildbucket_build_id)])


  # api.step('about to sleep for 10 minutes', cmd=['echo'])
  # api.step('sleep for 10 minutes', cmd=['sleep', '600'])


  # Hack start_dir to remove the "k" directory which is added by Kitchen.
  # Otherwise, we can't get to the CIPD packages, caches, and isolates which
  # were put into the task workdir.
  if api.path.c.base_paths['start_dir'][-1] == 'k':  # pragma: nocover
    api.path.c.base_paths['start_dir'] = api.path.c.base_paths['start_dir'][:-1]

  # Run Puppeteer tests in Docker container.
  golden_dir = api.path['start_dir'].join('buildbot').join('golden')
  with api.context(cwd=golden_dir, env={'DOCKER_CONFIG': '/home/chrome-bot/.docker'}):
    api.step('run puppeteer tests', cmd=['make', 'puppeteer-test'])

  # Upload any digests produced by Puppeteer tests to Gold.
  upload_digests_cmd = [
      'python',
      'upload-screenshots-to-gold.py',
      '--path_to_goldctl',
      '../../../goldctl/goldctl',  # This is [START_DIR]/goldctl/goldctl.
      '--revision', revision
  ]
  puppeteer_tests_dir = golden_dir.join('puppeteer-tests')
  with api.context(cwd=puppeteer_tests_dir):
    if is_trybot:
      cmd = upload_digests_cmd + ['--issue', issue,
                                  '--patch_set', patch_set,
                                  '--task_id', buildbucket_build_id]
      api.step('upload digests (tryjob)', cmd)
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
