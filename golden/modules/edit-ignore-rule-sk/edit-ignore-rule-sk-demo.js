import './index.js';
import { $$ } from 'common-sk/modules/dom';

import { manyParams} from "./test_data";

function newEditIgnoreRule(parentSelector) {
  const er = document.createElement('edit-ignore-rule-sk');
  er.params = manyParams;
  er.query = "configuration=Release";
  $$(parentSelector).appendChild(er);
}

newEditIgnoreRule(
    '#container');

