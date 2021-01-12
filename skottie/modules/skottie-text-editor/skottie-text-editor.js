/* eslint-disable */
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
 *
 */
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';

const originTemplate = (group) => {
  return html`
    <div class="text-element-item">
      <div class="text-element-label">
         Origin${group.items.length > 1 ? 's': ''}:
      </div>
        <ul>
          ${group.items.map((item) => html`
            <li class="text-element-origin">
              <b>${item.precompName}</b> > Layer ${item.layer.ind}
            </li>
          `)}
        </ul>
    </div>
  `
}

const template = (ele) => {
  return html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Text Editor</div>
      <div class="editor-header-separator"></div>
      <button class="editor-header-save-button" @click=${ele._toggleTextsCollapse}>${ele._state.areTextsCollapsed ? 'Ungroup Texts' : 'Group Texts' }</button>
      <button class="editor-header-save-button" @click=${ele._save}>Save</button>
    </header>
    <section>
      <ul class="text-container">
         ${ele._state.texts.map((item) => html`
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
                   @change=${ev => ele._onChange(ev, item)}
                   @input=${ev => ele._onChange(ev, item)}
                   .value=${item.text}
                 ></textarea>
               </div>
               <div>${originTemplate(item)}</div>

             </div>
           </li>`)}
      </ul>
    <section>
  </div>
  `;
};

class SkottieTextEditorSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      texts: [],
      areTextsCollapsed: true,
    };
  }

  findPrecompName(animation, precompId) {
    let animationLayers = animation.layers
    let comp = animationLayers.find(layer => layer.refId === precompId)
    if (comp) {
      return comp.nm
    }
    let animationAssets = animation.assets
    animationAssets.forEach(asset => {
      if (asset.layers) {
        asset.layers.forEach(layer => {
          if (layer.refId === precompId) {
            comp = layer
          }
        })
      }
    })
    if (comp) {
      return comp.nm
    }
    return 'not found'
  }

  buildTexts(animation) {
    let textsData = animation.layers
    .filter(layer => layer.ty === 5)
    .map(layer => ({layer: layer, parentId: '', precompName: 'Root'}))
    .concat(
      animation.assets
      .filter(asset => asset.layers)
      .reduce((accumulator, precomp)=>{
        accumulator = accumulator.concat(precomp.layers
          .filter(layer => layer.ty === 5)
          .map(layer => ({
            layer: layer,
            parentId: precomp.id,
            precompName: this.findPrecompName(animation, precomp.id),
          }))
        )
        return accumulator;
      }, [])
    )
    .reduce((accumulator, item, index)=>{
      let key = this._state.areTextsCollapsed
        ? item.layer.nm
        : (index + 1)
      if (!accumulator[key]) {
        accumulator[key] = {
          id: item.layer.nm,
          name: item.layer.nm,
          items: [],
          text: item.layer.t.d.k[0].s.t,
          precompName: item.precompName,
        }
      }

      accumulator[key].items.push(item);
      return accumulator;
    }, {})
    const texts = Object.keys(textsData)
      .map(key => textsData[key])
    
    this._state.texts = texts;
  }

  _save() {
    const applyHandler = this.apply;
    if (typeof applyHandler === 'function') {
      applyHandler(this._animation);
    }
  }

  _toggleTextsCollapse() {
    this._state.areTextsCollapsed = !this._state.areTextsCollapsed;
    this.buildTexts(this._animation);
    this._render();
  }

  _sanitizeText(text) {
    let sanitizedText = ''
    for (let i = 0; i < text.length; i += 1) {
      if (text.charCodeAt(i) === 10) {
        sanitizedText += String.fromCharCode(13);
      } else {
        sanitizedText += text.charAt(i);
      }
    }
    return sanitizedText;
  }

  _onChange(event, elem) {
    const text = this._sanitizeText(event.target.value);
    elem.text = text;
    elem.items.forEach(item => {
      item.layer.t.d.k[0].s.t = text;
    });
  }

  connectedCallback() {
    // console.log(this.animation);
    const animation = JSON.parse(JSON.stringify(this.animation));
    this.buildTexts(animation);
    this._animation = animation;
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
  }

  _render() {
    // console.log(this._state)
    render(template(this), this, { eventContext: this });
  }
}

define('skottie-text-editor', SkottieTextEditorSk);
