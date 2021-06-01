import './index';
import { deepCopy } from 'common-sk/modules/object';
import { clusterDiffJSON } from '../cluster-page-sk/test_data';
import { ClusterDiffNodeWithLabel, ClusterDigestsSk } from './cluster-digests-sk';

const ele = new ClusterDigestsSk();
document.body.querySelector('#cluster')!.appendChild(ele);

function setData(labels: boolean) {
  const nodes: ClusterDiffNodeWithLabel[] = deepCopy(clusterDiffJSON.nodes!);
  if (labels) {
    nodes.forEach((node, index) => {
      node.label = `node ${index}`;
    })
  }
  ele.setData(nodes, deepCopy(clusterDiffJSON.links!));
}

setData(false);

const labelsCheckBox = document.querySelector<HTMLInputElement>('#labels')!;
labelsCheckBox.addEventListener('change', (e: Event) => {
  setData(labelsCheckBox.checked);
});
