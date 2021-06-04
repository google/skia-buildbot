package validation

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.skia.org/infra/go/util"
)

// IsValidDigest returns true if the given string is a valid digest
// on the string level, i.e. it does not check whether we have
// actually seen the given hash but whether it complies with the format
// that we expect for a hash.
func IsValidDigest(hash string) bool {
	// Currently we expect all digests to be hex encoded MD5 hashes.
	if len(hash) != 2*md5.Size {
		return false
	}

	for _, c := range []byte(hash) {
		if ((c >= '0') && (c <= '9')) ||
			((c >= 'a') && (c <= 'f')) ||
			((c >= 'A') && (c <= 'F')) {
			continue
		}
		return false
	}
	return true
}

// Validation is a container to collect error messages during validation of a
// input with multiple fields.
type Validation []string

// StrValue validates a string value against containment in a set of options.
// Argument:
//     name: name of the field being validated.
//     val: value to be validated.
//     options: list of options, one of which value can contain.
//     defaultVal: default value in case val is empty. Can be equal to "".
// If there is a problem an error message will be added to the Validation object.
func (v *Validation) StrValue(name string, val *string, options []string, defaultVal string) {
	if *val == "" && defaultVal != "" {
		*val = defaultVal
		return
	}
	if !util.In(*val, options) {
		*v = append(*v, fmt.Sprintf("Field '%s' needs to be one of '%s'", name, strings.Join(options, ",")))
	}
}

// StrFormValue does the same as StrValue but extracts the given name from
// the request via r.FormValue(..).
func (v *Validation) StrFormValue(r *http.Request, name string, val *string, options []string, defaultVal string) {
	*val = r.FormValue(name)
	v.StrValue(name, val, options, defaultVal)
}

// Float64Value parses the value given in strVal and returns it. If strVal is empty
// the default value is returned.
func (v *Validation) Float64Value(name string, strVal string, defaultVal float64) float64 {
	if strVal == "" {
		return defaultVal
	}

	tempVal, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid float: %s", name, err))
	}
	return tempVal
}

// Int64Value parses the value given in strVal and returns it. If strVal is empty
// the default value is returned.
func (v *Validation) Int64Value(name string, strVal string, defaultVal int64) int64 {
	if strVal == "" {
		return defaultVal
	}

	tempVal, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid int: %s", name, err))
	}
	return tempVal
}

// Float64FormValue does the same as Float64Value but extracts the value from the request object.
func (v *Validation) Float64FormValue(r *http.Request, name string, defaultVal float64) float64 {
	return v.Float64Value(name, r.FormValue(name), defaultVal)
}

// Int64FormValue does the same as Int64Value but extracts the value from the request object.
func (v *Validation) Int64FormValue(r *http.Request, name string, defaultVal int64) int64 {
	return v.Int64Value(name, r.FormValue(name), defaultVal)
}

// Int64SliceValue parses a comma-separated list of int values and returns them.
func (v *Validation) Int64SliceValue(name string, strVal string, defaultVal []int64) []int64 {
	if strVal == "" {
		return defaultVal
	}

	splitVals := strings.Split(strVal, ",")
	ret := make([]int64, 0, len(splitVals))
	for _, oneStrVal := range splitVals {
		tempVal, err := strconv.ParseInt(oneStrVal, 10, 64)
		if err != nil {
			*v = append(*v, fmt.Sprintf("Field '%s' is not a valid list of comma separated integers: %s", name, err))
			return nil
		}
		ret = append(ret, tempVal)
	}
	return ret
}

// Int64SliceFormValue does the same as Int64SliceValue but extracts the given
// name from the request.
func (v *Validation) Int64SliceFormValue(r *http.Request, name string, defaultVal []int64) []int64 {
	return v.Int64SliceValue(name, r.FormValue(name), defaultVal)
}

// QueryFormValue extracts a URL-encoded query from the form values and decodes it.
// If the named field was not available in the given request an empty paramtools.ParamSet
// is returned. If an error occurs it will be added to the error list of the validation
// object.
func (v *Validation) QueryFormValue(r *http.Request, name string) map[string][]string {
	if q := r.FormValue(name); q != "" {
		ret, err := url.ParseQuery(q)
		if err != nil {
			*v = append(*v, fmt.Sprintf("Unable to parse query: %s. Error: %s", q, err))
			return nil
		}
		return ret
	}
	return map[string][]string{}
}

// Errors returns a concatenation of all error values accumulated in validation or nil
// if there were no errors.
func (v *Validation) Errors() error {
	if len(*v) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(*v, "\n"))
}
