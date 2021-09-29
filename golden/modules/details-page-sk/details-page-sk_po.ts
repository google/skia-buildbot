import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { DigestDetailsSkPO } from '../digest-details-sk/digest-details-sk_po';

/** A page object for the DetailsPageSk component. */
export class DetailsPageSkPO extends PageObject {
  get digestDetailsSkPO(): DigestDetailsSkPO {
    return this.poBySelector('digest-details-sk', DigestDetailsSkPO);
  }
}
