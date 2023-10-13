/**
 * @module skottie-slot-manager-sk
 * @description <h2><code>skottie-slot-manager-sk</code></h2>
 *
 * <p>
 *   A component meant to interface with a ManagedAnimation's SlotManager for
 *   property value swapping.
 * </p>
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { html } from 'lit-html';
import { SkottieColorEventDetail } from '../skottie-color-input-sk/skottie-color-input-sk';
import { colorToHex, hexToColor } from '../helpers/color';
import { ColorSlot } from './slot-info';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';
import { define } from '../../../elements-sk/modules/define';

export class SkottieSlotManagerSk extends ElementSk {
  private _player: SkottiePlayerSk | null = null;
  private colorSlots: ColorSlot[] = [];

  private static template = (ele: SkottieSlotManagerSk) => html`
    <div>
      <ul class="slots-container">
        ${ele.colorSlots.map((item: ColorSlot) => ele.colorSlot(item))}
      </ul>
    </div>
  `;

  private colorSlot = (cs: ColorSlot) => html`
    <div class="slot--color">
      <span class="slotID">${cs.id}</span>
      <skottie-color-input-sk
        .color=${cs.colorHex}
        @color-change=${(e: CustomEvent<SkottieColorEventDetail>) =>
          this.onColorSlotChange(e, cs.id)}>
      </skottie-color-input-sk>
    </div>
  `;

  private onColorSlotChange(
    e: CustomEvent<SkottieColorEventDetail>,
    sid: string
  ): void {
    if (!this._player) {
      return;
    }
    const color = hexToColor(e.detail.color);
    this._player.managedAnimation()?.setColorSlot(sid, color);
    this._render();
  }

  set player(value: SkottiePlayerSk) {
    this._player = value;

    this.colorSlots = [];
    const managedAnimation = this._player?.managedAnimation();
    if (managedAnimation) {
      const slotInfo = managedAnimation.getSlotInfo();
      for (const sid of slotInfo.colorSlotIDs) {
        const color = managedAnimation.getColorSlot(sid);
        if (color) {
          const colorHex = colorToHex(Array.from(color));
          this.colorSlots.push({ id: sid, colorHex: colorHex });
        }
      }
    }

    this._render();
  }

  constructor() {
    super(SkottieSlotManagerSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }
}

define('skottie-slot-manager-sk', SkottieSlotManagerSk);
