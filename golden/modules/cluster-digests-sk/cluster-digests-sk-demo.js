import './index';
import { $$ } from 'common-sk/modules/dom';
import { clusterDiffJSON } from './test_data';

const ele = document.createElement('cluster-digests-sk');
ele.setData(clusterDiffJSON.nodes, clusterDiffJSON.links);
$$('#cluster').appendChild(ele);
