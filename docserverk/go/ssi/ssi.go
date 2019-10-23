// ssi implements a simple server side include mechanism that allows to replace
// special tags with the output of function calls.
package ssi

/*
 Server side includes (SSI) map simple custom tags to function calls.
 Such a tag looks like this:

	 <ssi:tag_id key1=val1 key2=val2>

	where 'tag_id' is the name of the tag and the key/value pairs are the
	parameters of the tag.

	The ProcessSSI function in this package takes a HTML document and
	replaces all SSI tags with the output of the function that has
	been associated with the respective tag_id.

	This covers any use case where we want to generate dynamic content and
	leave the placement and configuration of it up to the user.

	For example the tag implemented below allows the user to write

		<ssi:listgcs path=skia-bucket/path/to/dir>

	which would then be replaced with a listing of specified GCS location.
*/

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

// ssiProcessFn is a function that is executed to produce the output for a custom tag.
type ssiProcessFn func(params map[string]string) ([]byte, error)

var (
	// processFNs maps the id of an SSI function to the function implementing the tag.
	processFNs = map[string]ssiProcessFn{}

	// findSSIRegEx finds the ssi tags and extracts the relevant information as groups.
	findSSIRegEx = regexp.MustCompile(`<ssi:(?P<tag_id>[a-zA-Z]+)\s*(?P<params>.*?)>`)
)

// Init initializes the package with a client to access GCS and the URL of the
// target repo which is used to generate links to commits.
func Init(repoURL string, client *storage.Client) {
	// Set up the tag to list files from GCS.
	lister := &gceFolderListing{
		client:  client,
		repoURL: strings.TrimRight(repoURL, "/") + "/",
	}
	processFNs["listgce"] = lister.generateFolderListing
}

// ProcessSSI finds SSI tags in the given body and resolves them.
// It returns the body with all SSI tags replaced with the output of
// the functions that were registered for them.
func ProcessSSI(body []byte) ([]byte, error) {
	// find whether there are any includes
	indices := findSSIRegEx.FindAllSubmatchIndex(body, -1)
	if len(indices) > 0 {
		// parts collects the portions of the original that we keep and the
		// newly generated output in the right order.
		parts := make([][]byte, 0, len(indices)*2+1)
		currStart := 0
		totalLen := 0

		// Iterate over all tags we have found.
		for _, match := range indices {
			// Use the start/end indices of the tag to keep track of the parts of the
			// document that we want to keep. See the 'prefix' variable below.
			tagStart, tagEnd := match[0], match[1]
			id := string(body[match[2]:match[3]])
			paramStr := string(body[match[4]:match[5]])

			// Parse the tag parameters, which are simply space separated 'key=value' pairs.
			params, err := parseParams(paramStr)
			if err != nil {
				return nil, skerr.Fmt("Error parsing params in tag '%s': %s", string(body[tagStart:tagEnd]), err)
			}

			// Find the function registered for the given tag.
			fn, ok := processFNs[id]
			if !ok {
				return nil, skerr.Fmt("Unable to find function for tag '%s'", string(body[tagStart:tagEnd]))
			}

			// Run the function and insert the returned byte slice instead of the tag.
			fill, err := fn(params)
			if err != nil {
				return nil, skerr.Fmt("Error processing tag '%s': %s", string(body[tagStart:tagEnd]), err)
			}
			prefix := body[currStart:tagStart]
			parts = append(parts, prefix, fill)
			currStart = tagEnd
			totalLen += len(prefix) + len(fill)
		}

		// Add the suffix after the last tag and re-assemble the parts of
		// the original document and the newly generated parts into one.
		parts = append(parts, body[currStart:])
		body = make([]byte, 0, totalLen)
		for _, part := range parts {
			body = append(body, part...)
		}
	}

	return body, nil
}

// parseParams parses the key=value pairs that make up the parameters of the tag.
func parseParams(paramsStr string) (map[string]string, error) {
	paramsStr = strings.TrimSpace(paramsStr)
	if paramsStr == "" {
		return map[string]string{}, nil
	}

	kvPairs := strings.Fields(paramsStr)
	ret := make(map[string]string, len(kvPairs))
	for _, kvPair := range kvPairs {
		kv := strings.SplitN(kvPair, "=", 2)
		k := strings.TrimSpace(kv[0])
		v := ""
		if k == "" {
			return nil, skerr.Fmt("Missing key in parameters '%s'", paramsStr)
		}
		if len(kv) == 2 {
			v = strings.TrimSpace(kv[1])
		}
		ret[k] = v
	}
	return ret, nil
}

// gceFolderList allows to list the contents of folders in GCS. Its
// 'generateFolderListing' function implements the ssiProcessFn signature and
// is used to implement the <ssi:listgcs> tag. That tag takes one
// parameter which is the path (format: bucket/prefix) of the GCS, e.g.
//
//      <ssi:listgcs path=mybucket/somefolder>
//
// would create a table that contains the listing of all public files in
// 'bucket' that is in folder 'somefolder'. The files are assumed to have
// these commit related meta data attached to them (relating to the commit
// that generated the file):
//     - commit : the commit hash
//     - commit_message : the commit message
//     - date: the date of the commit
//
type gceFolderListing struct {
	client  *storage.Client
	repoURL string
}

// Note: The variable below are not constants because they are swapped out during
// testing to make verification simpler.
var (
	// listGCSTagSnippet is the code snippet that encapsulates that listing of the
	// files in a GCS folder.
	listGCSTagSnippet = `<table><thead><tr>
  		<th>Created</th>
	  	<th>Link</th>
	  	<th>Commit Date</th>
	  	<th>Commit</th>
	  	<th>Commit Message</th>
		</tr></thead>
		<tbody>
		%s
		</tbody>
	</table>`

	// listGCSURLTmpl is the template to generate URLs in GCS from bucket and path strings.
	listGCSURLTmpl = "https://storage.cloud.google.com/%s/%s"

	// nListGCSEntries is the number of entries we'll show in the listing.
	nListGCSEntries = 10

	// gcsListingRowTmpl is the template to generate a single entry in the GCS folder listing.
	gceListingRowTmpl = template.Must(template.New("row").Parse(`<tr>
		<td>{{.created}}</td>
		<td><a href="{{.url}}">{{.name}}</a></td>
		<td>{{.date}}</td>
		<td>{{.commit}}</td>
		<td><a href="{{.commit_url}}">{{.commit_message}}</a></td>
	</tr>`))
)

// generateFolderListing generates HTML that is inserted into the document instead of the
// tag that is tied to this function.
func (g *gceFolderListing) generateFolderListing(params map[string]string) ([]byte, error) {
	gcsPath, ok := params["path"]
	if !ok {
		return nil, skerr.Fmt("No 'path' parameter provided in tag.")
	}

	bucket, path := gcs.SplitGSPath(gcsPath)
	path = strings.TrimRight(path, "/") + "/"

	iter := g.client.Bucket(bucket).Objects(context.Background(), &storage.Query{Prefix: path, Delimiter: "/"})
	items := []map[string]interface{}{}

	for {
		objAttrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, skerr.Fmt("Error retrieving object attributes: %s", err)
		}

		// Add entry if it's not a folder, if we have meta data and if the item is public.
		if objAttrs.Prefix == "" && objAttrs.Metadata != nil && isPublic(objAttrs.ACL) {
			// Copy the meta data attributes.
			attrs := map[string]interface{}{}
			for k, v := range objAttrs.Metadata {
				attrs[k] = v
			}

			// Generate some additional attributes. Mark the URLs as trusted so they
			// are not further escaped.
			attrs["name"] = objAttrs.Name[strings.LastIndex(objAttrs.Name, "/")+1:]
			attrs["url"] = template.HTML(fmt.Sprintf(listGCSURLTmpl, objAttrs.Bucket, objAttrs.Name))
			attrs["commit_url"] = template.HTML(g.repoURL + "+/" + attrs["commit"].(string))
			attrs["created"] = objAttrs.Created.Format(time.RFC3339)
			items = append(items, attrs)
		}
	}

	// Sort the folder entries in reverse chronological order by their creation time.
	sort.Slice(items, func(i, j int) bool {
		return items[i]["created"].(string) > items[j]["created"].(string)
	})
	items = items[:util.MinInt(len(items), nListGCSEntries)]
	var buf bytes.Buffer
	for _, attrs := range items {
		fmt.Println(attrs["created"])
		if err := gceListingRowTmpl.Execute(&buf, attrs); err != nil {
			return nil, skerr.Fmt("Unable to execute template: %s", err)
		}
	}
	return []byte(fmt.Sprintf(listGCSTagSnippet, buf.String())), nil
}

// isPublic returns true if one in the given ACLRules allows all users to
// read the content.
func isPublic(aclRules []storage.ACLRule) bool {
	for _, rule := range aclRules {
		if rule.Entity == storage.AllUsers && rule.Role == storage.RoleReader {
			return true
		}
	}
	return false
}
