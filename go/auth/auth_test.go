package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestSkolo(t *testing.T) {
	src := `{"access_token":"ya29.c.ElutBRJ3iCAwM3CqFCh9HqGtVc2fQdhwEtQQbYXInp_0DM1Cxw6lWyiiQIMjpOIQrdnnuei7qCeZexI0iK_LG6-iOX-tGzb2_5_lvMJWHet5_qwDeOfncv3zwJOP","token_type":"Bearer","expiry":"2018-04-30T09:56:47.330462568-04:00"}`
	r := strings.NewReader(src)
	var tok oauth2.Token
	err := json.NewDecoder(r).Decode(&tok)
	assert.NoError(t, err)
	fmt.Println(tok)
}
