import './index';
import { $$ } from 'common-sk/modules/dom';
import { clusterDiffJSON } from '../cluster-page-sk/test_data';

const ele = document.createElement('cluster-digests-sk');
ele.setData(clusterDiffJSON.nodes, clusterDiffJSON.links);
$$('#cluster').appendChild(ele);
