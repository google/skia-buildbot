// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package client

import (
	"crypto/x509"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

const pkey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAvc6v42I4badqYA+IF9dMB838Q2l2IflSpA8xSC5O7XrDwa1R
YCqPq+MOIIaMUgqBMJz0OmNyQkbtRLq3Qu4Q44UIbdqyy34rj7kcw/9t/K9x+2ne
Phx0tfdz+5Lj6UiRRI7FRCi9cs+mgSQquCDaBW8J5l3lCZEne8fpHPO3Hxl+dkUX
0Y8T2ZCsn19hnV7Z7wbfN1dUuRihXD+UwN2axoqZ0EJ2GNSLYAz3HHkKc6ELM1Lx
biOD9Jxw8wb+5VnpIuR3l426Fcux9EQGewLZFTxjRS7DRPL/9L0xE+yRJ/I04UyJ
v4Ws98fYp/vAM922Wt21P9Py6vgn+Xjyz2AoyQIDAQABAoIBABKQhq+Mycwf1c2z
dzItwqf4w7WsOPu1sRmOytkuflXH7iGhXBY103uSZ61Su6LCmEQy9chINcK5wTc5
s/b95fT67Aoim94/Zu9VwbSB5TYTyug2QKB+lAPAQj3W7ifBC0RTWoQCIBV8reJv
sSX1QJ3LcIJxqJc49U2sDebhB4YpAv7xmy4IfpqX+0iURtXrgBmp0hWKTQb7kRdG
BycDU9/AadgkI1PEhRdbfJ4VHFKxkeSRwPyp1UKvzydfe7Nw2HWlflEH4fZCc29x
AM52K5zN/7ns/xEz9XPOUG0/pBcXeQNA5rbTGoZrhQda/aBWbI9TYGWh7XZFvx5y
vZ/xlckCgYEA6ULnJYn+DDUfa1eKwYsYvzf82pbfBvrpTsIKi9h0CWy7GltYuSJk
6yt6pzEmAeV+eY6/F8pPxN0TTZHQAVcRHiMbazmLiaLUeuZCvIZwU54ttyENC2+k
fLUlt3a5eiPKBZEPGx++HuESWVY0LYk8hcg9koc4+AIsiifXz9kgzRcCgYEA0E9h
Dn1qWEXYxzV6Efcux62d9P8vwRx+ZWtbCdMleosVAWQ4RtS8HKfjYvJdPcKJlaGO
b7VyYbJzayPs0skhayEYXajDhwcykxNCYJTXxSqh3Hf4yEeRLquDLW468a9tRc8q
Q2wv+lav7ZeW+Db35fq0mEHRaUn0iXFiq9c1JR8CgYEA16ocrk98TGsdRpCk4Lcr
RTiNlsihIgIAjenH+G5DMqeOAhts15beObR0bXp6ioxVuCvrsCJESF6iRzjGWUbX
s8Z/xk5pHfMngw27rDScTCNWXxe2yNkK+qY9XffuGuhWE3l/vvNFQ6WS4nhaO7PD
+mkdzIkredoAtieKWEiHFDcCgYBQetqcpoe3owSlslt/JWjFbKZiSVVB3qhWtqtt
mE4akjGDYBz+AKLMz3BighDUE5zkWo6VShzu8er1seOFbH+kzByF0vX37Sf0+rPi
bJ8QZfAzJYbQmhXVWh5MJxJO3d/x4KALfHjs1yERQkfpjhMonzu2t3cYnqIDl/Lv
QS4fMQKBgFx5masOJqHNx/HDOLtOvO8MeKbZgb2wzrBteJBs/KyFjP4fzZZseiVV
67XuwVxrLup7KzUaHK8PysA+ZgiT4ZlvyX+J+pFZA2XPtKTKCA3bKYtIG2JF5W1v
uHXl2FV53+kI2rF188v3jbuUhK0FrsUEXpN8C+dotMMLCLakbNXP
-----END RSA PRIVATE KEY-----`

// Cert for luci-token-server-test-1.fake.domain.
const cert = `-----BEGIN CERTIFICATE-----
MIIEFjCCAv6gAwIBAgICEAAwDQYJKoZIhvcNAQELBQAwYDELMAkGA1UEBhMCVVMx
EzARBgNVBAgMCkNhbGlmb3JuaWExDTALBgNVBAcMBEJsYWgxEjAQBgNVBAoMCVN0
dWZmIEluYzEZMBcGA1UEAwwQRmFrZSBDQTogZmFrZS5jYTAeFw0xNjA0MDkwNDIx
MTJaFw0xNzA0MTkwNDIxMTJaMHQxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxp
Zm9ybmlhMQ0wCwYDVQQHDARCbGFoMRIwEAYDVQQKDAlTdHVmZiBJbmMxLTArBgNV
BAMMJGx1Y2ktdG9rZW4tc2VydmVyLXRlc3QtMS5mYWtlLmRvbWFpbjCCASIwDQYJ
KoZIhvcNAQEBBQADggEPADCCAQoCggEBAL3Or+NiOG2namAPiBfXTAfN/ENpdiH5
UqQPMUguTu16w8GtUWAqj6vjDiCGjFIKgTCc9DpjckJG7US6t0LuEOOFCG3asst+
K4+5HMP/bfyvcftp3j4cdLX3c/uS4+lIkUSOxUQovXLPpoEkKrgg2gVvCeZd5QmR
J3vH6Rzztx8ZfnZFF9GPE9mQrJ9fYZ1e2e8G3zdXVLkYoVw/lMDdmsaKmdBCdhjU
i2AM9xx5CnOhCzNS8W4jg/SccPMG/uVZ6SLkd5eNuhXLsfREBnsC2RU8Y0Uuw0Ty
//S9MRPskSfyNOFMib+FrPfH2Kf7wDPdtlrdtT/T8ur4J/l48s9gKMkCAwEAAaOB
xTCBwjAJBgNVHRMEAjAAMBEGCWCGSAGG+EIBAQQEAwIFoDAzBglghkgBhvhCAQ0E
JhYkT3BlblNTTCBHZW5lcmF0ZWQgQ2xpZW50IENlcnRpZmljYXRlMB0GA1UdDgQW
BBQf/Xtn7MQpybujv9/54DxdiNDKFzAfBgNVHSMEGDAWgBRhO7licgHsGIwDmWmP
zL+oymoPHjAOBgNVHQ8BAf8EBAMCBeAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG
AQUFBwMEMA0GCSqGSIb3DQEBCwUAA4IBAQA9YXLIMwJbfQMMpTxPZLQoiqbG0fPB
xuSBGNYd/US6NIWLOg/v5tfN2GB+RAuB1Rz12eS+TmN7+A/lfNx0dFYwcfeOz05g
jQMwgUDmlnicMqENd0pswccS/mci215addFq6Wknti+To+TST0Ci5zmIt2fbBjmI
VRAWsPfLInwtW94S54UF38n2gp3iXizQLG2urSqotPsWIiyO+f2M3Q2ki3fDzimj
EyA+GFsGD6l0nQUySNyk2xE4S5CHOyLG0qWOsaJsEkTMnN+lrUh1bLUcI3bvVpVP
uwi+mmV6pbwEPKYNHpxHXSbEFnWwnZm1OtM28sP9O0D94XzRq2OfWiiD
-----END CERTIFICATE-----`

func TestX509Signer(t *testing.T) {
	Convey("works", t, func() {
		ctx := context.Background()

		signer := X509Signer{
			PrivateKeyPEM:  []byte(pkey),
			CertificatePEM: []byte(cert),
		}

		algo, err := signer.Algo(ctx)
		So(err, ShouldBeNil)
		So(algo, ShouldEqual, x509.SHA256WithRSA)

		der, err := signer.Certificate(ctx)
		So(err, ShouldBeNil)
		_, err = x509.ParseCertificate(der) // valid cert
		So(err, ShouldBeNil)

		blob, err := signer.Sign(ctx, []byte("blah"))
		So(err, ShouldBeNil)
		So(len(blob), ShouldEqual, 256)
	})
}
