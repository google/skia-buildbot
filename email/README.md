# email

The email service consolidates sending emails into a single service.

## API

The client POST's the message in
[RFC2822](https://datatracker.ietf.org/doc/html/rfc2822) format to this service
running only on internally exposed port.

The server will parse the `From:` line from the sent message and use that to
determine which account to use.

I.e. the format of the POST body will look like this:

~~~
From: <alerts@skia.org>
To: some-list@example.com
Subject: Alert
Content-Type: text/html; charset=UTF-8

<html>
<body>
...
</body>
</html>
~~~

From the extracted email address the server will look up the correct
credentials, stored in Google Secret Manager, and will then send the email using
the GMail API using the selected credentials.

## Implementation

Secrets for each GMail account are held in [Google Secret
Manager](https://cloud.google.com/secret-manager) where they secret key will be
a flattened email address. This is, Secret Manager keys must match:

        `[[a-zA-Z_0-9-]+]`

So we'll convert all chars outside that list into an underscore. So the key for
the account `alerts@skia.org` will have a key `alerts_skia_org`.
