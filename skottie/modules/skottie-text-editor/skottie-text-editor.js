/**
 * @module skottie-text-editor
 * @description <h2><code>skottie-text-editor</code></h2>
 *
 * <p>
 *   A skottie text editor
 * </p>
 *
 *
 * @evt apply - This event is generated when the user presses Apply.
 *         The updated json is available in the event detail.
 *
 * @attr animation the animation json.
 *         At the moment it only reads it at load time.
 *
 */
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import { ifDefined } from 'lit-html/directives/if-defined';

const originTemplateElement = (item) => html`
  <li class="text-element-origin">
    <b>${item.precompName}</b> > Layer ${item.layer.ind}
  </li>
`;

const originTemplate = (group) => html`
  <div class="text-element-item">
    <div class="text-element-label">
       Origin${group.items.length > 1 ? 's' : ''}:
    </div>
      <ul>
        ${group.items.map(originTemplateElement)}
      </ul>
  </div>
`;

const textElement = (item, element) => html`
  <li class="text-element">
    <div class="text-element-wrapper">
      <div class="text-element-item">
        <div class="text-element-label">
          Layer name:
        </div>
        <div>
          ${item.name}
        </div>
      </div>
      <div class="text-element-item">
        <div class="text-element-label">
          Layer text:
        </div>
        <textarea class="text-element-input"
          @change=${(ev) => element._onChange(ev, item)}
          @input=${(ev) => element._onChange(ev, item)}
          maxlength=${ifDefined(item.maxChars)}
          .value=${item.text}
        ></textarea>
      </div>
      <div>${originTemplate(item)}</div>
    </div>
  </li>
`;

const template = (ele) => html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Text Editor</div>
      <div class="editor-header-separator"></div>
      <button class="editor-header-save-button" @click=${ele._toggleTextsCollapse}>${ele._state.areTextsCollapsed ? 'Ungroup Texts' : 'Group Texts'}</button>
      <button class="editor-header-save-button" @click=${ele._save}>Save</button>
    </header>
    <section>
      <ul class="text-container">
         ${ele._state.texts.map((item) => textElement(item, ele))}
      </ul>
    <section>
  </div>
`;

const LAYER_TEXT_TYPE = 5;
const COMP_ROOT_NAME = 'Root';
const LINE_FEED = 10;
const FORM_FEED = 13;

class SkottieTextEditorSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      texts: [],
      areTextsCollapsed: true,
    };
    this._animation = null;
    this._originalAnimation = null;
  }

  findPrecompName(animation, precompId) {
    const animationLayers = animation.layers;
    let comp = animationLayers.find((layer) => layer.refId === precompId);
    if (comp) {
      return comp.nm;
    }
    const animationAssets = animation.assets;
    animationAssets.forEach((asset) => {
      if (asset.layers) {
        asset.layers.forEach((layer) => {
          if (layer.refId === precompId) {
            comp = layer;
          }
        });
      }
    });
    if (comp) {
      return comp.nm;
    }
    return 'not found';
  }

  _buildTexts(animation) {
    const textsData = animation.layers // we iterate all layer at the root layer
      .filter((layer) => layer.ty === LAYER_TEXT_TYPE) // we filter all layers of type text
      .map((layer) => ({ layer: layer, parentId: '', precompName: COMP_ROOT_NAME })) // we map them to some extra data
      .concat(
        animation.assets // we iterate over the assets of the animation looking for precomps
          .filter((asset) => asset.layers) // we filter assets that of type precomp (by querying if they have a layers property)
          .reduce((accumulator, precomp) => { // we flatten into a single array layers from multiple precomps
            accumulator = accumulator.concat(precomp.layers
              .filter((layer) => layer.ty === LAYER_TEXT_TYPE) // we filter all layers of type text
              .map((layer) => ({ // we map them to some extra data
                layer: layer,
                parentId: precomp.id,
                precompName: this.findPrecompName(animation, precomp.id),
              })));
            return accumulator;
          }, []),
      )
      .reduce((accumulator, item, index) => { // this creates a dictionary with all available texts
        const key = this._state.areTextsCollapsed
          ? item.layer.nm // if texts are collapsed the key will be the layer name (nm)
          : (index + 1); // if they are not collapse we use the index as key to be unique
        if (!accumulator[key]) {
          accumulator[key] = {
            id: item.layer.nm,
            name: item.layer.nm,
            items: [],
            // this property is the text string of a text layer.
            // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
            text: item.layer.t.d.k[0].s.t,
            maxChars: item.layer.t.d.k[0].s.mc, // Max characters text document attribute
            precompName: item.precompName,
          };
        }

        accumulator[key].items.push(item);
        return accumulator;
      }, {});
    const texts = Object.keys(textsData)
      .map((key) => textsData[key]); // we map the dictionary back to an array to get the final texts to render

    this._state.texts = texts;
  }

  _save() {
    this.dispatchEvent(new CustomEvent('apply', {
      detail: {
        texts: this._state.texts,
      },
    }));
  }

  _toggleTextsCollapse() {
    this._state.areTextsCollapsed = !this._state.areTextsCollapsed;
    this._buildTexts(this._animation);
    this._render();
  }

  _sanitizeText(text) {
    let sanitizedText = '';
    for (let i = 0; i < text.length; i += 1) {
      if (text.charCodeAt(i) === LINE_FEED) {
        sanitizedText += String.fromCharCode(FORM_FEED);
      } else {
        sanitizedText += text.charAt(i);
      }
    }
    return sanitizedText;
  }

  _onChange(event, elem) {
    const text = this._sanitizeText(event.target.value);
    elem.text = text;
    elem.items.forEach((item) => {
      // this property is the text string of a text layer.
      // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
      item.layer.t.d.k[0].s.t = text;
    });
  }

  _updateAnimation(animation) {
    if (animation && this._originalAnimation !== animation) {
      const clonedAnimation = JSON.parse(JSON.stringify(animation));
      this._buildTexts(clonedAnimation);
      this._animation = clonedAnimation;
      this._originalAnimation = animation;
      this._render();
    }
  }

  /** @prop animation {Object} new animation to traverse. */
  set animation(val) {
    this._updateAnimation(val);
  }

  connectedCallback() {
    this._updateAnimation(this.animation);
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
}

define('skottie-text-editor', SkottieTextEditorSk);
