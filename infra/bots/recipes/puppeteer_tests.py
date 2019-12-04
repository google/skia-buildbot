DEPS = [
    'recipe_engine/step',
]


def RunSteps(api):
  api.step('say hello', cmd=['echo', 'hello'])
  api.step('say bye', cmd=['echo', 'bye'])
  api.step('exit with nonzero code', cmd=['python', '-c', 'import sys; sys.exit(1)'])
  api.step('say something after exiting with nonzero code', cmd=['echo', 'hi'])


def GenTests(api):
  yield (
      api.test('Hello-World')
  )
