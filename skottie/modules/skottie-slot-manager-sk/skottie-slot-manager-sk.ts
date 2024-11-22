/**
 * @module skottie-slot-manager-sk
 * @description <h2><code>skottie-slot-manager-sk</code></h2>
 *
 * <p>
 *   A component meant to interface with a ManagedAnimation's SlotManager for
 *   property value swapping.
 * </p>
 *
 * @evt slot-manager-change - This event is generated when the user updates
 *         a slot value.
 *         The updated json is available in the event detail.
 *
 * @attr animation the animation json.
 *         At the moment it only reads it at load time.
 *
 */

import { html, TemplateResult } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SkottieColorEventDetail } from '../skottie-color-input-sk/skottie-color-input-sk';
import { colorToHex, hexToColor } from '../helpers/color';
import { ColorSlot, ScalarSlot, Vec2Slot } from './slot-info';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';
import { define } from '../../../elements-sk/modules/define';
import { SkottieVec2EventDetail } from './skottie-vec2-input-sk';
import { LottieAnimation } from '../types';
import { SkottieTemplateEventDetail } from '../skottie-color-manager-sk/skottie-color-manager-sk';

// without this import, the vec2 input div doesn't populate and vec 2 slots don't render
import './skottie-vec2-input-sk';
import {
  replaceColorSlot,
  replaceScalarSlot,
  replaceVec2Slot,
  replaceImageSlot,
} from './slot-replace';

export class SkottieSlotManagerSk extends ElementSk {
  private _player: SkottiePlayerSk | null = null;

  private _resourceList: string[] = [];

  private colorSlots: ColorSlot[] = [];

  private scalarSlots: ScalarSlot[] = [];

  private vec2Slots: Vec2Slot[] = [];

  private imageSlots: string[] = [];

  private _animation: LottieAnimation | null = null;

  private originalAnimation: LottieAnimation | null = null;

  private static template = (ele: SkottieSlotManagerSk) => html`
    <div class="wrapper">${ele.renderView()}</div>
  `;

  public hasSlots(): boolean {
    return (
      this.colorSlots.length !== 0 ||
      this.scalarSlots.length !== 0 ||
      this.vec2Slots.length !== 0 ||
      this.imageSlot.length !== 0
    );
  }

  private renderView(): TemplateResult {
    if (
      this.colorSlots.length ||
      this.scalarSlots.length ||
      this.vec2Slots.length ||
      this.imageSlots.length
    ) {
      return this.renderSlotManager(this);
    }
    return this.renderUnslotted();
  }

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

  private scalarSlot = (ss: ScalarSlot) => html`
    <div class="slot--scalar">
      <span class="slotID">${ss.id}</span>
      <div class="text-box">
        <input
          type="number"
          class="text-box--input"
          @change=${(e: Event) => this.onScalarSlotChange(e, ss.id)}
          value=${ss.scalar}
          required />
      </div>
    </div>
  `;

  private vec2Slot = (vs: Vec2Slot) => html`
    <div class="slot--vec2">
      <skottie-vec2-input-sk
        .label=${vs.id}
        .x=${vs.x}
        .y=${vs.y}
        @vec2-change=${(e: CustomEvent<SkottieVec2EventDetail>) =>
          this.onVec2SlotChange(e)}>
      </skottie-vec2-input-sk>
    </div>
  `;

  private imageSlot = (is: string) => html`
    <div class="slot--image">
      <span class="slotID">${is}</span>
      <select @change=${(e: Event) => this.onImageSlotChange(e, is)}>
        <option>--Select from uploaded</option>
        ${this._resourceList.map((item: string) => this.imageOption(item))}
      </select>
    </div>
  `;

  private imageOption = (name: string) => html`
    <option value="${name}">${name}</option>
  `;

  private renderUnslotted(): TemplateResult {
    return html`
      <div class="no-manager">
        <div class="info-box">
          <span class="icon-sk info-box--icon">info</span>
          <span class="info-box--description">
            Add properties to AE's Essential Graphics window to create slots.
            Ensure that that slots are being exported correctly by checking
            exporter settings.
          </span>
        </div>
      </div>
    `;
  }

  private renderSlotManager(ele: SkottieSlotManagerSk): TemplateResult {
    return html`
      <div class="wrapper">
        <ul class="slots-container">
          ${ele.colorSlots.map((item: ColorSlot) => ele.colorSlot(item))}
          ${ele.scalarSlots.map((item: ScalarSlot) => ele.scalarSlot(item))}
          ${ele.vec2Slots.map((item: Vec2Slot) => ele.vec2Slot(item))}
          ${ele.imageSlots.map((item: string) => ele.imageSlot(item))}
        </ul>
      </div>
    `;
  }

  private onColorSlotChange(
    e: CustomEvent<SkottieColorEventDetail>,
    sid: string
  ): void {
    if (!this._animation) {
      return;
    }

    const { color, opacity } = e.detail;

    const newAnimation = replaceColorSlot(color, opacity, sid, this._animation);

    this.dispatchEvent(
      new CustomEvent<SkottieTemplateEventDetail>('slot-manager-change', {
        detail: {
          animation: newAnimation,
        },
      })
    );
    this._render();
  }

  private onScalarSlotChange(e: Event, sid: string): void {
    if (!this._animation) {
      return;
    }

    const target = e.target as HTMLInputElement;
    const newVal = Number(target.value);

    const newAnimation = replaceScalarSlot(newVal, sid, this._animation);

    this.dispatchEvent(
      new CustomEvent<SkottieTemplateEventDetail>('slot-manager-change', {
        detail: {
          animation: newAnimation,
        },
      })
    );

    this._render();
  }

  private onVec2SlotChange(e: CustomEvent<SkottieVec2EventDetail>): void {
    if (!this._animation) {
      return;
    }

    const newAnimation = replaceVec2Slot(
      [e.detail.x, e.detail.y],
      e.detail.label,
      this._animation
    );

    this.dispatchEvent(
      new CustomEvent<SkottieTemplateEventDetail>('slot-manager-change', {
        detail: {
          animation: newAnimation,
        },
      })
    );

    this._render();
  }

  private onImageSlotChange(e: Event, sid: string): void {
    if (!this._animation) {
      return;
    }

    const target = e.target as HTMLInputElement;
    const newAnimation = replaceImageSlot(target.value, sid, this._animation);

    this.dispatchEvent(
      new CustomEvent<SkottieTemplateEventDetail>('slot-manager-change', {
        detail: {
          animation: newAnimation,
        },
      })
    );

    this._render();
  }

  set player(value: SkottiePlayerSk) {
    this._player = value;

    this.colorSlots = [];
    this.scalarSlots = [];
    this.vec2Slots = [];
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
      for (const sid of slotInfo.scalarSlotIDs) {
        const scalar = managedAnimation.getScalarSlot(sid);
        if (scalar !== null && scalar !== undefined) {
          this.scalarSlots.push({ id: sid, scalar: scalar });
        }
      }
      for (const sid of slotInfo.vec2SlotIDs) {
        const vec2 = managedAnimation.getVec2Slot(sid);
        if (vec2) {
          this.vec2Slots.push({ id: sid, x: vec2[0], y: vec2[1] });
        }
      }
      this.imageSlots = slotInfo.imageSlotIDs;
    }

    this._render();
  }

  set resourceList(value: string[]) {
    this._resourceList = value;

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

  private updateAnimation(animation: LottieAnimation): void {
    if (animation && this.originalAnimation !== animation) {
      const clonedAnimation = JSON.parse(
        JSON.stringify(animation)
      ) as LottieAnimation;
      this._animation = clonedAnimation;
      this.originalAnimation = animation;
      this._render();
    }
  }

  set animation(val: LottieAnimation) {
    this.updateAnimation(val);
  }
}

define('skottie-slot-manager-sk', SkottieSlotManagerSk);
