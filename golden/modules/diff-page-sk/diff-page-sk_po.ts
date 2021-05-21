import {PageObject} from '../../../infra-sk/modules/page_object/page_object';
import {DigestDetailsSkPO} from '../digest-details-sk/digest-details-sk_po';

/** A page object for the DiffPageSk component. */
export class DiffPageSkPO extends PageObject {
  get digestDetailsSkPO(): DigestDetailsSkPO {
    return this.poBySelector('digest-details-sk', DigestDetailsSkPO);
  }
}
