AutoRoll
========

AutoRoll is a program which creates and manages DEPS rolls of Skia into Chrome.


### AutoRoll ###
It needs the following project level metadata set:

    metadata.COOKIESALT
    metadata.CLIENT_ID
    metadata.CLIENT_SECRET

The client_id and client_secret come from here:

    https://console.developers.google.com/project/31977622648/apiui/credential

Look for the Client ID that has a Redirect URI for mon.skia.org.

For 'cookiesalt' search for 'skiamonitor' in valentine.
