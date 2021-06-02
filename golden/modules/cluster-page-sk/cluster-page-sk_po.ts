import {PageObject} from '../../../infra-sk/modules/page_object/page_object';
import {SearchControlsSkPO} from '../search-controls-sk/search-controls-sk_po';
import {ParamSetSkPO} from '../../../infra-sk/modules/paramset-sk/paramset-sk_po';
import {ClusterDigestsSkPO} from '../cluster-digests-sk/cluster-digests-sk_po';

/** A page object for the ClusterPageSk component. */
export class ClusterPageSkPO extends PageObject {
  get searchControlsSkPO(): SearchControlsSkPO {
    return this.poBySelector('.page-container > search-controls-sk', SearchControlsSkPO);
  }

  get clusterDigestsSkPO(): ClusterDigestsSkPO {
    return this.poBySelector('.page-container > cluster-digests-sk', ClusterDigestsSkPO);
  }

  get paramSetSkPO(): ParamSetSkPO {
    return this.poBySelector('.page-container > paramset-sk', ParamSetSkPO);
  }
}
