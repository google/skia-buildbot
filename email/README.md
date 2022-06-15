# email

The email service consolidates sending emails into a single service.

## API

The client POST's the message in
[RFC2822](https://datatracker.ietf.org/doc/html/rfc2822) format to `/send`  on
this service running only on an internally exposed port.

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

## Client

The `emailclient.Client` is a replacement for `email.Gmail` that will only
need minor changes to a `SendWithMarkup()` call. It is almost a drop-in
replacement for `email.Gmail.SendWithMarkup()`
with the following changes:

 - The 'from' email address must be supplied.
 - The function no longer returns a message id.