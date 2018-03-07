exports.defineTags = function(dictionary) {
  console.log("loaded");
  dictionary.defineTag('evt', {
    canHaveName: true,
    onTagged: function(doclet, tag) {
      console.log("doclet:", doclet);
      console.log("tag:", tag);
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
}

const eventRows = (events) => events.map( (e) => `
  <tr>
    <td>${e.name}</td>
    <td><pre>${e.description}</pre></td>
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
          ${eventRows(e.doclet.webevents).join('')}
        </table>
      `;
      e.doclet.description = `${e.doclet.description}
      ${table}`;
    }
  }
}
