import './index';
import { $$ } from 'common-sk/modules/dom';
import { manyParams } from '../shared_demo_data';

Date.now = () => Date.parse('2020-02-01T00:00:00Z');

function newEditIgnoreRule(parentSelector, query = '', expires = '', note = '') {
  const ele = document.createElement('edit-ignore-rule-sk');
  ele.paramset = manyParams;
  ele.query = query;
  ele.expires = expires;
  ele.note = note;
  $$(parentSelector).appendChild(ele);
}

newEditIgnoreRule('#empty');
newEditIgnoreRule('#filled',
  'alpha_type=Opaque&compiler=GCC&cpu_or_gpu_value=Adreno540&cpu_or_gpu_value=GTX660&'
  + 'name=01_original.jpg_0.333&source_options=codec_animated_kNonNative_unpremul',
  '2020-02-03T00:00:00Z', 'this is my note');
$$('#filled query-sk .selection div:nth-child(1)').click();
newEditIgnoreRule('#missing');
$$('#missing edit-ignore-rule-sk').verifyFields();

newEditIgnoreRule('#partial_custom_values');
$$('#partial_custom_values input.custom_key').value = 'oops';
$$('#partial_custom_values button.add_custom').click();
