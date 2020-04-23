/**
 * @fileoverview A custom element that loads the CT pending tasks queue and
 * displays it as a table.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp, taskDescriptors, getTimestamp, getCtDbTimestamp,
} from '../ctfe_utils';

const template = (ele) => html`
<style>
    #suggestions_div {
      display: block;
      border: 1px solid;
      position: absolute;
      z-index: 1;
    }

    li.selected {
      background-color: var(--secondary);
      color: var(--on-secondary);
    }
    li:hover {
      background-color: var(--primary-variant);
    }
</style>
    <input autocomplete=off @input=${ele._refresh} @change=${ele._change} @keyup=${ele._keyup} @focus=${ele._focus} @blur=${ele._blur}></input>
    <div ?hidden=${!ele.show_suggestions()} @click=${ele._suggestion_click}>
      <ul style="list-style-type:none;">
      ${ele._buildSuggestionList()}
      </ul> 
    </div>
`;

const optionTemplate = (option) => html`
<li class=suggestion>${option}</li>
`;
const selectedOptionTemplate = (option) => html`
<li class="suggestion selected">${option}</li>
`;

function hideDialog(e) {
  if (e.target.classList.contains('dialog-background')) {
    e.target.classList.add('hidden');
  }
}

function formatTask(task) {
  return JSON.stringify(task, null, 4);
}
const DOWN_ARROW = 40;
const UP_ARROW = 38;
const ENTER = 13;

define('suggest-input-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._options = ['golang','c++','JS','Python2.7','Python3','IronPython'];
    this.displayOptionsOnFocus = true;
    this._value = '';
    this._suggestions = this._options;
    this._suggestion_selected = -1;
    this._input = $$('input', this);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get value() {
    return this._value;
  }
  set value(v) {
    this._value = v;
  }

  _buildSuggestionList() {
    let templates = [];
    for (let i = 0; i < this._suggestions.length; ++i) {
      let s = this._suggestions[i];
      templates.push( i === this._suggestion_selected ? selectedOptionTemplate(s) : optionTemplate(s));
    }
    return templates;
  }
  _refresh(e) {
    let v = e.target.value;
    var re;
    try {
      re = new RegExp(v, "i");  // case-insensitive.
    } catch (e) {
      // If the user enters an invalid expression, just use substring
      // match.
      re = new Object();
      re.test = function(str) {
        return str.indexOf(v) != -1;
      };
    }
    console.log('using:' + v);
    this._suggestion_selected = -1;
    this._suggestions = this._options.filter(s => re.test(s))
    this._render();
  }

  _keyup(e) {
    // Allow the user to scroll through suggestions using arrow keys.
    let len = this._suggestions.length;
    if (e.keyCode === DOWN_ARROW && len > 0) {
      this._suggestion_selected = (this._suggestion_selected + 1) % len
      console.log('selected DOWN puts us at:' + this._suggestion_selected);
      this._render();
    } else if (e.keyCode === UP_ARROW && this._suggestions.length > 0) {
      this._suggestion_selected = (this._suggestion_selected  + len - 1) % len
      console.log('selected UP puts us at:' + this._suggestion_selected);
      this._render();
    }
  }

  _suggestion_click(e) {
    let selection = e.target;
    selection = Array.from(selection.parentNode.children).indexOf(selection)
    if (selection != this._suggestion_selected) {
      this._suggestion_selected = selection;
      this._render();
    }
  }

  _blur(e) {
    var blurredElem = e.relatedTarget;
    if (!blurredElem) {
      blurredElem = e.detail.sourceEvent.relatedTarget;
    }
    if (blurredElem && blurredElem.classList.contains("suggestion")) {
      return;
    }

    let selection = this._suggestion_selected;
    if (selection > -1) {
      // If the enter key is pressed while one of the suggestions is
      // highlighted, we accept that selection.
      this._input.value = this._suggestions[selection];
      this._commit();
    } else if (this._options.includes(this._input.value)) {
      // The user manually typed one of the acceptable values.
      this._commit();
    } else if (this.acceptCustomValue) {
      // If the enter key is pressed without highlighting a suggestion
      // then accept the current input value.
      this._commit();
    } else {
      // User entered an invalid selection, and custom values are not
      // allowed.
      this._input.value = "";
      this._commit();
    }

  }
/*
  _onFocus(e) {
    if (this.displayOptionsOnFocus && (
            this.value === '' || this.value === undefined)) {
      this.value = '';
      this._keyup(e);
    }
  }

  _value_changed() {
    // This event fires when we set this.value or when our parent sets it.
    // In the latter case, we need to propagate the change down to our
    // child paper-input.
    if (this._committing) {
      // The event fired because _commit() was called. Don't propagate the
      // change or we'll get an infinite loop.
      this._committing = false;
    } else {
      // Our parent set this.value. Propagate the change.
      this.$.input.value = this.value;
    }
  }

  _change(e) {
    // This event fires when the user "finishes" typing in the text box.
    // This occurs when the enter key is pressed, or when the text box
    // loses focus. We only want our parent to receive a change event when
    // the user has finished typing, so we need to handle the case in which
    // this event fires because the user clicked one of the suggestions.
    e.preventDefault();
    e.stopPropagation();
    var v = this.$.input.value;
    if (this.autocomplete && this.autocomplete.indexOf(v) === -1) {
      // If we're autocompleting, don't fire the change event unless we
      // chose a suggested value.
    } else {
      this._commit()
    }
  }

  _commit() {
    // The user has "finished" typing. This occurs when the user clicks
    // a suggestion, or, once the user has typed out an exact match to one
    // of the suggestions, they hit the enter key or click out of the text
    // box. Set this.value and trigger a "change" event.
    this._committing = true;
    this._suggestions = [];
    this.$.suggestions_box.selected = -1;
    this.value = this.$.input.value;
    this.fire("change", {"value": this.value});
  }

  _keyup(e) {
      } else {
      }
    }
  }

  _suggestion_changed() {
    if (this._arrow_key_pressed) {
      // This event fires while we're arrowing through the options. We don't
      // want to accept a suggestion while this is happening, so ignore the
      // event in that case.
      this._arrow_key_pressed = false;
    } else {
      // The user has clicked on a suggestion. Accept it.
      var selected = this.$.suggestions_box.selectedItem.value;
      this.$.input.value = selected;
      this._commit();
    }
  }
*/
  show_suggestions() {
    console.log(this._suggestions && this._suggestions.length > 0);
    return this._suggestions && this._suggestions.length > 0;
  }
});


/*
  <script>
  (function(){
    var DOWN_ARROW = 40;
    var UP_ARROW = 38;
    var ENTER = 13;

    Polymer({
      is:"autocomplete-input-sk",

      properties: {
        autocomplete: {
          type: Array,
        },

        label: {
          type: String,
        },

        value: {
          type: String,
          value: "",
          notify: true,
          observer: "_value_changed",
        },

        displayOptionsOnFocus: {
          type: Boolean,
          value: false,
        },

        acceptCustomValue: {
          type: Boolean,
          value: false,
        },

        _arrow_key_pressed: {
          type: Boolean,
          value: false,
        },

        _committing: {
          type: Boolean,
          value: false,
        },

        _suggestions: {
          type: Array,
          value: function() {
            return [];
          },
        },
      },

      _onFocus: function(e) {
        if (this.displayOptionsOnFocus && (
                this.value === '' || this.value === undefined)) {
          this.value = '';
          this._keyup(e);
        }
      },

      _value_changed: function() {
        // This event fires when we set this.value or when our parent sets it.
        // In the latter case, we need to propagate the change down to our
        // child paper-input.
        if (this._committing) {
          // The event fired because _commit() was called. Don't propagate the
          // change or we'll get an infinite loop.
          this._committing = false;
        } else {
          // Our parent set this.value. Propagate the change.
          this.$.input.value = this.value;
        }
      },

      _change: function(e) {
        // This event fires when the user "finishes" typing in the text box.
        // This occurs when the enter key is pressed, or when the text box
        // loses focus. We only want our parent to receive a change event when
        // the user has finished typing, so we need to handle the case in which
        // this event fires because the user clicked one of the suggestions.
        e.preventDefault();
        e.stopPropagation();
        var v = this.$.input.value;
        if (this.autocomplete && this.autocomplete.indexOf(v) === -1) {
          // If we're autocompleting, don't fire the change event unless we
          // chose a suggested value.
        } else {
          this._commit()
        }
      },

      _commit: function() {
        // The user has "finished" typing. This occurs when the user clicks
        // a suggestion, or, once the user has typed out an exact match to one
        // of the suggestions, they hit the enter key or click out of the text
        // box. Set this.value and trigger a "change" event.
        this._committing = true;
        this._suggestions = [];
        this.$.suggestions_box.selected = -1;
        this.value = this.$.input.value;
        this.fire("change", {"value": this.value});
      },

      _keyup: function(e) {
        if (e.type === "blur") {
          // Ignore the blur event if it was caused by clicking a suggestion.
          var blurredElem = e.relatedTarget;
          if (!blurredElem) {
            blurredElem = e.detail.sourceEvent.relatedTarget;
          }
          if (blurredElem && blurredElem.classList.contains("suggestion")) {
            return;
          }
        }
        if (this.autocomplete) {
          // Allow the user to scroll through suggestions using arrow keys.
          if (e.keyCode === DOWN_ARROW && this._suggestions.length > 0) {
            this._arrow_key_pressed = true;
            this.$.suggestions_box.selected = this.$.suggestions_box.selected + 1;
          } else if (e.keyCode === UP_ARROW && this._suggestions.length > 0) {
            this._arrow_key_pressed = true;
            this.$.suggestions_box.selected = this.$.suggestions_box.selected - 1;
          } else if (e.type === "blur" || e.keyCode === ENTER) {
            if (this.$.suggestions_box.selected > -1) {
              // If the enter key is pressed while one of the suggestions is
              // highlighted, we accept that selection.
              this.$.input.value = this.$.suggestions_box.selectedItem.value;
              this._commit();
            } else if (this.autocomplete.includes(this.$.input.value)) {
              // The user manually typed one of the acceptable values.
              this._commit();
            } else if (this.acceptCustomValue) {
              // If the enter key is pressed without highlighting a suggestion
              // then accept the current input value.
              this._commit();
            } else {
              // User entered an invalid selection, and custom values are not
              // allowed.
              this.$.input.value = "";
              this._commit();
            }
          } else {
            // The user has entered text. Recompute autocomplete suggestions.
            var v = this.$.input.value;
            this._suggestions = [];
            // Give suggestions on empty input only if displayOptionsOnFocus
            // is true.
            if (v != "" || this.displayOptionsOnFocus) {
              // If possible, treat partially-entered input as a regular
              // expression.
              var re;
              try {
                re = new RegExp(v, "i");  // case-insensitive.
              } catch (e) {
                // If the user enters an invalid expression, just use substring
                // match.
                re = new Object();
                re.test = function(str) {
                  return str.indexOf(v) != -1;
                };
              }
              // Include anything in the autocomplete list which matches the
              // text in the box.
              for (var i = 0; i < this.autocomplete.length; ++i) {
                if (re.test(this.autocomplete[i])) {
                  this.push("_suggestions", this.autocomplete[i]);
                }
              }
            }
            // If the user types an exact match to one of the suggestions,
            // remove the suggestions box.
            if (this._suggestions.length === 1 && this._suggestions[0] === v) {
              this._suggestions = [];
            }
            // Ensure that no suggestions are selected after recomputing.
            this.$.suggestions_box.selected = -1;
          }
        }
      },

      _suggestion_changed: function() {
        if (this._arrow_key_pressed) {
          // This event fires while we're arrowing through the options. We don't
          // want to accept a suggestion while this is happening, so ignore the
          // event in that case.
          this._arrow_key_pressed = false;
        } else {
          // The user has clicked on a suggestion. Accept it.
          var selected = this.$.suggestions_box.selectedItem.value;
          this.$.input.value = selected;
          this._commit();
        }
      },

      _show_suggestions: function() {
        console.log(this._suggestions && this._suggestions.length > 0);
        return this._suggestions && this._suggestions.length > 0;
      },
    });
  })()
  </script>
  */
