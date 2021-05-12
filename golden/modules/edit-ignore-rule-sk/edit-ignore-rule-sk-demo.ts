import './index';
import { $$ } from 'common-sk/modules/dom';
import { manyParams } from '../shared_demo_data';
import { EditIgnoreRuleSk } from './edit-ignore-rule-sk';
import { EditIgnoreRuleSkPO } from './edit-ignore-rule-sk_po';

Date.now = () => Date.parse('2020-02-01T00:00:00Z');

function newEditIgnoreRule(
    parentSelector: string,
    query = '',
    expires = '',
    note = ''): {el: EditIgnoreRuleSk, po: EditIgnoreRuleSkPO} {
  const el = new EditIgnoreRuleSk();
  el.paramset = manyParams;
  el.query = query;
  el.expires = expires;
  el.note = note;
  $$(parentSelector)!.appendChild(el);
  return {el: el, po: new EditIgnoreRuleSkPO(el)};
}

async function populate() {
   newEditIgnoreRule('#empty');

  const {po: filledPO} = newEditIgnoreRule('#filled',
    'alpha_type=Opaque&compiler=GCC&cpu_or_gpu_value=Adreno540&cpu_or_gpu_value=GTX660&'
    + 'name=01_original.jpg_0.333&source_options=codec_animated_kNonNative_unpremul',
    '2020-02-03T00:00:00Z', 'this is my note');
  await (await filledPO.querySkPO)?.clickKey('alpha_type');

  const {el: missing} = newEditIgnoreRule('#missing');
  missing.verifyFields();

  const {po: partialCustomValuesPO} = newEditIgnoreRule('#partial_custom_values');
  await partialCustomValuesPO.setCustomKey('oops');
  await partialCustomValuesPO.clickAddCustomParamBtn();
}

populate();
