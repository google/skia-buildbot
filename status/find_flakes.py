import datetime
import json
import urllib2


DAYS = 1
TIME_FORMAT = '%Y-%m-%dT%H:%M:%S.000000Z'
URL_TMPL = 'https://task-scheduler.skia.org/json/tasks/search?time_start=%s&time_end=%s'


def get_tasks(start, end, params):
  url = URL_TMPL % (start.strftime(TIME_FORMAT), end.strftime(TIME_FORMAT))
  for k, v in params.iteritems():
    url += '&%s=%s' % (k, v)
  resp = urllib2.urlopen(url)
  return json.load(resp)


def get_flake_data(start, end):
  flakes = get_tasks(start, end, {
    'attempt': '1',
    'status': 'SUCCESS',
  })
  all_tasks = get_tasks(start, end, {})
  num_flakes = len(flakes)
  num_total  = len(all_tasks)
  percent_flaky = float(num_flakes)/float(num_total)*100.0
  with open('flaky_tasks.json', 'wb') as f:
    json.dump(flakes, f, indent=2, sort_keys=True)
  return num_flakes, num_total, percent_flaky


def main():
  now = datetime.datetime.utcnow()
  end = now
  for i in range(DAYS):
    start = end - datetime.timedelta(days=1)
    flakes, total, percent = get_flake_data(start, end)
    print '%d flakes in %d total tasks (%2f%%) in (%s - %s)' % (flakes, total, percent, start, end)
    end = start


if __name__ == '__main__':
  main()
