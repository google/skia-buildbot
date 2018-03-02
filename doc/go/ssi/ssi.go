package ssi

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"google.golang.org/api/iterator"
)

type ssiProcessFn func(params map[string]string) []byte

var (
	processFNs = map[string]ssiProcessFn{}
	findRegEx  = regexp.MustCompile(`<ssi:(\S+)\s*(.*)>`)
)

func Init(client *storage.Client) {
	processFNs["listgce"] = (&gceFolderListing{client: client}).genListing
}

func ProcessSSI(body []byte) []byte {
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
			fill := processFNs[id](parseParams(paramStr))
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

	return body
}

func parseParams(paramsStr string) map[string]string {
	kvPairs := strings.Fields(paramsStr)
	ret := make(map[string]string, len(kvPairs))
	for _, kvPair := range kvPairs {
		kv := strings.Split(kvPair, "=")
		ret[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return ret
}

type gceFolderListing struct {
	client *storage.Client
}

const (
	rowTmpl = `<tr>
		<td>{{.commit}}</td>
		<td>{{.commit_message}}</td>
		<td>{{.date}}</td>
		<td><a href="{{.url}}">{{.title}}</a></td>
	</tr>`
	snippet = "<table>%s</table>"
	urlTmpl = "https://storage.cloud.google.com/%s/%s"
)

var tmpl = template.Must(template.New("row").Parse(rowTmpl))

func (g *gceFolderListing) genListing(params map[string]string) []byte {
	gcsPath := params["path"]
	bucket, path := gcs.SplitGSPath(gcsPath)
	path = strings.TrimRight(path, "/") + "/"
	iter := g.client.Bucket(bucket).Objects(context.Background(), &storage.Query{Prefix: path})
	N_ENTRIES := 10
	var buf bytes.Buffer
	counter := 0
	for {
		objAttrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			panic(err.Error())
		}
		if isPublic(objAttrs.ACL) {
			metaData := objAttrs.Metadata
			if metaData != nil {
				metaData["name"] = objAttrs.Name
				metaData["url"] = fmt.Sprintf(urlTmpl, objAttrs.Bucket, objAttrs.Name)
				if err := tmpl.Execute(&buf, metaData); err != nil {
					panic(err.Error())
				}
			}
		}

		counter++
		if counter == N_ENTRIES {
			break
		}

	}
	return []byte(fmt.Sprintf(snippet, buf.String()))
}

func isPublic(aclRules []storage.ACLRule) bool {
	for _, rule := range aclRules {
		if rule.Entity == storage.AllUsers && rule.Role == storage.RoleReader {
			return true
		}
	}
	return false
}
