import upload_bench_results
import unittest

class ConfigParseTest(unittest.TestCase):

  def test_viewport(self):
    result=upload_bench_results._ParseConfig('viewport_100x100')
    self.assertEqual(result, {
      'viewport': '100x100',
    })

  def test_enums(self):
    for config in ['8888', 'gpu', 'msaa4', 'nvprmsaa4', 'nvprmsaa16']:
      result = upload_bench_results._ParseConfig(config)
      self.assertEqual(result, {
        'config': config
      })
    for bbh in ['rtree', 'quadtree', 'grid']:
      result = upload_bench_results._ParseConfig(bbh)
      self.assertEqual(result, {
        'bbh': bbh
      })
    for mode in ['simple', 'record', 'tile']:
      result = upload_bench_results._ParseConfig(mode)
      self.assertEqual(result, {
        'mode': mode
      })

  def test_badparams(self):
    result = upload_bench_results._ParseConfig('foo')
    self.assertEqual(result, {
      'badParams': foo
    })


if __name__ == '__file__':
  unittest.main()
