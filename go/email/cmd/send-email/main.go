package main

/**
 * Accepts a RFC2822-formatted file (or stdin) and sends an email.
 */

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
)

var (
	fileName = flag.String("file", "", "File containing an RFC2822-formatted email to send.")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	var contents []byte
	var err error
	if *fileName == "" {
		contents, err = io.ReadAll(os.Stdin)
	} else {
		contents, err = os.ReadFile(*fileName)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	_, to, subject, body, err := email.ParseRFC2822Message(contents)
	if err != nil {
		sklog.Fatal(err)
	}

	client, err := email.NewClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	resp, err := client.SendMail(ctx, &email.SendMailRequest{
		//Sender:   from,
		To:       to,
		Subject:  subject,
		TextBody: body,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println(resp.MessageId)
}
