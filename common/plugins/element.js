exports.defineTags = function(dictionary) {
  dictionary.defineTag('evt', {
    canHaveName: true,
    onTagged: function(doclet, tag) {
      if (!doclet.webevents) {
        doclet.webevents= [];
      }

      doclet.webevents.push(
        {
          name: tag.value.name,
          description: tag.value.description || '',
        }
      );
    }
  });

  dictionary.defineTag('attr', {
    canHaveName: true,
    onTagged: function(doclet, tag) {
      if (!doclet.attrs) {
        doclet.attrs= [];
      }

      doclet.attrs.push(
        {
          name: tag.value.name,
          description: tag.value.description || '',
        }
      );
    }
  });
}

const rows = (events) => events.map( (e) => `
  <tr>
    <td>${e.name}</td>
    <td>${e.description}</td>
  </tr>
`);

exports.handlers = {
  newDoclet: function(e) {
    if (e.doclet.webevents) {
      const table = `<h5>Events</h5>
        <table class=params>
          <thead>
            <th>Name</th>
            <th>Description</th>
          </thead>
          ${rows(e.doclet.webevents).join('')}
        </table>
      `;
      e.doclet.description = `${e.doclet.description || ''}
      ${table}`;
    }
    if (e.doclet.attrs) {
      const table = `<h5>Attributes</h5>
        <table class=params>
          <thead>
            <th>Name</th>
            <th>Description</th>
          </thead>
          ${rows(e.doclet.attrs).join('')}
        </table>
      `;
      e.doclet.description = `${e.doclet.description || ''}
      ${table}`;
    }
  }
}
