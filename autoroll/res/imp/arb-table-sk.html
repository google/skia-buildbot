<!--
  The common.js file must be included before this file.
  This is an HTML Import-able file that contains the definition
  of the following elements:

    <arb-table-sk>

  To use this file import it:

    <link href="/res/imp/arb-table-sk.html" rel="import" />

  Usage:

    <arb-table-sk></arb-table-sk>

  Properties:
    rollers: Array of strings; names of all autorollers.

  Methods:
    None.

  Events:
    None.
-->

<link rel="import" href="/res/common/imp/styles-sk.html">

<dom-module id="arb-table-sk">
  <style include="styles-sk">
  div.table{
    margin: 20px;
  }
  </style>
  <template>
    <div class="table">
      <div class="th">Roller ID</div>
      <div class="th">Current Mode</div>
      <div class="th">Num Behind</div>
      <div class="th">Num Failed</div>
      <template is="dom-repeat" items="{{_rollers}}">
        <div class="tr">
          <div class="td"><a href="/r/{{item}}">{{_name(item)}}</a></div>
          <div class="td">{{_mode(item)}}</div>
          <div class="td">{{_numBehind(item)}}</div>
          <div class="td">{{_numFailed(item)}}</div>
        </div>
      </template>
    </div>
  </template>
  <script>
  Polymer({
    is: "arb-table-sk",

    properties: {
      // input
      rollers: {
        type: Array,
        observer: "_update",
      },

      // private
      _rollers: {
        type: Array,
      },
      _statuses: {
        type: Object,
      },
    },

    _mode: function(roller) {
      return this._statuses[roller].mode;
    },

    _name: function(roller) {
      return this._statuses[roller].childName + " into " + this._statuses[roller].parentName;
    },

    _numBehind: function(roller) {
      return this._statuses[roller].numBehind;
    },

    _numFailed: function(roller) {
      return this._statuses[roller].numFailed;
    },

    _update: function() {
      var url = "/json/all";
      sk.get(url).then(JSON.parse).then(function(data) {
        this.set("_statuses", data);
        this.set("_rollers", this.rollers);
      }.bind(this)).catch(function(msg) {
        sk.errorMessage("Failed to load data: " + msg);
      }.bind(this));
    },
  });
  </script>
</dom-module>
