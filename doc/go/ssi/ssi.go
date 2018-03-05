package ssi

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/storage"
	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

type ssiProcessFn func(params map[string]string) ([]byte, error)

var (
	processFNs = map[string]ssiProcessFn{}
	findRegEx  = regexp.MustCompile(`<ssi:(\S+)\s*(.*?)>`)
)

func Init(repoURL string, client *storage.Client) {
	lister := &gceFolderListing{
		client:  client,
		repoURL: strings.TrimRight(repoURL, "/") + "/",
	}
	processFNs["listgce"] = lister.genListing
}

func ProcessSSI(body []byte) ([]byte, error) {
	// find whether there are any includes
	indices := findRegEx.FindAllSubmatchIndex(body, -1)
	if len(indices) > 0 {
		parts := make([][]byte, 0, len(indices)*2+1)
		currStart := 0
		totalLen := 0
		for _, match := range indices {
			tagStart, tagEnd := match[0], match[1]
			id := string(body[match[2]:match[3]])
			paramStr := string(body[match[4]:match[5]])

			params, err := parseParams(paramStr)
			if err != nil {
				return nil, sklog.FmtErrorf("Error parsing params in tag '%s': %s", string(body[tagStart:tagEnd]), err)
			}

			fill, err := processFNs[id](params)
			if err != nil {
				return nil, sklog.FmtErrorf("Error processing tag '%s': %s", string(body[tagStart:tagEnd]), err)
			}
			prefix := body[currStart:tagStart]
			parts = append(parts, prefix, fill)
			currStart = tagEnd
			totalLen += len(prefix) + len(fill)
		}
		parts = append(parts, body[currStart:])
		body = make([]byte, 0, totalLen)
		for _, part := range parts {
			body = append(body, part...)
		}
	}

	return body, nil
}

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
			return nil, sklog.FmtErrorf("Missing key in parameters '%s'", paramsStr)
		}
		if len(kv) == 2 {
			v = strings.TrimSpace(kv[1])
		}
		ret[k] = v
	}
	return ret, nil
}

type gceFolderListing struct {
	client  *storage.Client
	repoURL string
}

const (
	rowTmpl = `<tr>
		<td>{{.created}}</td>
		<td><a href="{{.url}}">{{.name}}</a></td>
		<td>{{.date}}</td>
		<td>{{.commit}}</td>
		<td><a href="{{.commit_url}}">{{.commit_message}}</a></td>
	</tr>`
	snippet = `<table><thead><tr>
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
	urlTmpl = "https://storage.cloud.google.com/%s/%s"
)

var tmpl = template.Must(template.New("row").Parse(rowTmpl))

func (g *gceFolderListing) genListing(params map[string]string) ([]byte, error) {
	sklog.Infof("Listing: %s", spew.Sdump(params))

	gcsPath := params["path"]
	bucket, path := gcs.SplitGSPath(gcsPath)
	path = strings.TrimRight(path, "/") + "/"
	sklog.Infof("path: '%s / %s'", bucket, path)

	N_ENTRIES := 10
	iter := g.client.Bucket(bucket).Objects(context.Background(), &storage.Query{Prefix: path, Delimiter: "/"})
	items := []map[string]string{}

	for {
		objAttrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, sklog.FmtErrorf("Error retrieving object attributes: %s", err)
		}

		sklog.Infof("NAME: '%s' - '%s'", objAttrs.Name, objAttrs.Prefix)

		// Add entry if it's not a folder, if we have meta data and if the item is public.
		if objAttrs.Prefix == "" && objAttrs.Metadata != nil && isPublic(objAttrs.ACL) {
			attrs := util.CopyStringMap(objAttrs.Metadata)
			attrs["name"] = objAttrs.Name[strings.LastIndex(objAttrs.Name, "/")+1:]
			attrs["url"] = fmt.Sprintf(urlTmpl, objAttrs.Bucket, objAttrs.Name)
			attrs["commit_url"] = g.repoURL + "+/" + attrs["commit"]
			attrs["created"] = objAttrs.Created.Format(time.RFC3339)
			items = append(items, attrs)
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i]["created"] > items[j]["created"] })
	items = items[:util.MinInt(len(items), N_ENTRIES)]
	var buf bytes.Buffer
	for _, attrs := range items {
		fmt.Println(attrs["created"])
		if err := tmpl.Execute(&buf, attrs); err != nil {
			return nil, sklog.FmtErrorf("Unable to execute template: %s", err)
		}
	}
	return []byte(fmt.Sprintf(snippet, buf.String())), nil
}

func isPublic(aclRules []storage.ACLRule) bool {
	for _, rule := range aclRules {
		if rule.Entity == storage.AllUsers && rule.Role == storage.RoleReader {
			return true
		}
	}
	return false
}
