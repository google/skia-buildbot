// This is a jsdoc plugin that adds support for both @evt and @attr
// tags on a class, where @evt documents an event the custome element may
// generate, and @attr documents an attribute the custom element recognizes.



// The tags are defined here, and we use the onTagged callback to
// store the name and description associated with each tag.
exports.defineTags = function(dictionary) {
  dictionary.defineTag('evt', {
    canHaveName: true,
    onTagged: function(doclet, tag) {
      if (!doclet.webevents) {
        doclet.webevents= [];
      }

      doclet.webevents.push({
        name: tag.value.name,
        description: tag.value.description || '',
      });
    }
  });

  dictionary.defineTag('attr', {
    canHaveName: true,
    onTagged: function(doclet, tag) {
      if (!doclet.attrs) {
        doclet.attrs= [];
      }

      doclet.attrs.push({
        name: tag.value.name,
        description: tag.value.description || '',
      });
    }
  });
}

const rows = (rowData) => rowData.map((e) => `
  <tr>
    <td>${e.name}</td>
    <td>${e.description}</td>
  </tr>
`);

const section = (title, rowData) => `<h5>${title}</h5>
    <table class=params>
    <thead>
      <th>Name</th>
      <th>Description</th>
    </thead>
    ${rows(rowData).join('')}
    </table>
`;

// The handlers look for doclets with data we stored from our custom
// tags and emit HTML for the data we found.
exports.handlers = {
  newDoclet: function(e) {
    if (e.doclet.webevents) {
      e.doclet.description = `
        ${e.doclet.description || ''}
        ${section('Events', e.doclet.webevents)}
      `;
    }
    if (e.doclet.attrs) {
      e.doclet.description = `
        ${e.doclet.description || ''}
        ${section('Attributes', e.doclet.attrs)}
      `;
    }
  }
}
