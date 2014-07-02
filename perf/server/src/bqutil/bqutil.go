// Package bqutil provides helpers for working with BigQuery.
package bqutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

import (
	"code.google.com/p/google-api-go-client/bigquery/v2"
)

// RowIter is a utility for reading data from a BigQuery query response.
//
// RowIter will iterate over all the results, even if they span more than one
// page of results. It automatically uses page tokens to iterate over all the
// pages to retrieve all results.
type RowIter struct {
	response      *bigquery.GetQueryResultsResponse
	jobId         string
	service       *bigquery.Service
	nextPageToken string
	row           int
}

// poll until the job is complete.
func (r *RowIter) poll() error {
	var queryResponse *bigquery.GetQueryResultsResponse
	for {
		var err error
		queryCall := r.service.Jobs.GetQueryResults("google.com:chrome-skia", r.jobId)
		if r.nextPageToken != "" {
			queryCall.PageToken(r.nextPageToken)
		}
		queryResponse, err = queryCall.Do()
		if err != nil {
			return err
		}
		if queryResponse.JobComplete {
			break
		}
		time.Sleep(time.Second)
	}
	r.nextPageToken = queryResponse.PageToken
	r.response = queryResponse
	return nil
}

// NewRowIter starts a query and returns a RowIter for iterating through the
// results.
func NewRowIter(service *bigquery.Service, query string) (*RowIter, error) {
	job := &bigquery.Job{
		Configuration: &bigquery.JobConfiguration{
			Query: &bigquery.JobConfigurationQuery{
				Query: query,
			},
		},
	}
	jobResponse, err := service.Jobs.Insert("google.com:chrome-skia", job).Do()
	if err != nil {
		return nil, err
	}

	r := &RowIter{
		jobId:   jobResponse.JobReference.JobId,
		service: service,
		row:     -1, // Start at -1 so the first call to Next() puts us at the 0th Row.
	}
	return r, r.poll()
}

// Next moves to the next row in the response and returns true as long as data
// is availble, returning false when the end of the results are reached.
//
// Calling Next() the first time actually points the iterator at the first row,
// which makes it possible to use Next if a for loop:
//
//    for iter.Next() { ... }
//
func (r *RowIter) Next() bool {
	r.row++
	if r.row >= len(r.response.Rows) {
		if r.nextPageToken != "" {
			r.poll()
			r.row = 0
			return len(r.response.Rows) > 0
		} else {
			return false
		}
	}
	return true
}

// DecodeParams pulls all the values in the params record out as a map[string]string.
//
// The schema for each table has a nested record called 'params' that contains
// various axes along which queries could be built, such as the gpu the test was
// run against. Pull out the entire record as a generic map[string]string.
func (r *RowIter) DecodeParams() map[string]string {
	row := r.response.Rows[r.row]
	schema := r.response.Schema
	params := map[string]string{}
	for i, cell := range row.F {
		if cell.V != nil {
			name := schema.Fields[i].Name
			if strings.HasPrefix(name, "params_") {
				params[strings.TrimPrefix(name, "params_")] = cell.V.(string)
			}
		}
	}
	return params
}

// Decode uses struct tags to decode a single row into a struct.
//
// For example, given a struct:
//
//   type A struct {
//     Name string   `bq:"name"`
//     Value float64 `bq:"measurement"`
//   }
//
// And a BigQuery table that contained two columns named "name" and
// "measurement". Then calling Decode as follows would parse the column values
// for "name" and "measurement" and place them in the Name and Value fields
// respectively.
//
//   a = &A{}
//   iter.Decode(a)
//
// Implementation Details:
//
//   If a tag names a column that doesn't exist, the field is merely ignored,
//   i.e. it is left unchanged from when it was passed into Decode.
//
//   Not all columns need to be tagged in the struct.
//
//   The decoder doesn't handle nested structs, only the top level fields are decoded.
//
//   The decoder only handles struct fields of type string, int, int32, int64,
//   float, float32 and float64.
func (r *RowIter) Decode(s interface{}) error {
	row := r.response.Rows[r.row]
	schema := r.response.Schema
	// Collapse the data in the row into a map[string]string.
	rowMap := map[string]string{}
	for i, cell := range row.F {
		if cell.V != nil {
			rowMap[schema.Fields[i].Name] = cell.V.(string)
		}
	}

	// Then iter over the fields of 's' and set them from the row data.
	sv := reflect.ValueOf(s).Elem()
	st := sv.Type()
	for i := 0; i < sv.NumField(); i++ {
		columnName := st.Field(i).Tag.Get("bq")
		if columnValue, ok := rowMap[columnName]; ok {
			switch sv.Field(i).Kind() {
			case reflect.String:
				sv.Field(i).SetString(columnValue)
			case reflect.Float32, reflect.Float64:
				f, err := strconv.ParseFloat(columnValue, 64)
				if err != nil {
					return err
				}
				sv.Field(i).SetFloat(f)
			case reflect.Int32, reflect.Int64:
				parsedInt, err := strconv.ParseInt(columnValue, 10, 64)
				if err != nil {
					return err
				}
				sv.Field(i).SetInt(parsedInt)
			default:
				return fmt.Errorf("Can't decode into field of type: %s %s", columnName, sv.Field(i).Kind())
			}
		}
	}
	return nil
}
