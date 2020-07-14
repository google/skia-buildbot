import fetchMock from 'fetch-mock';
import { ARBScaffoldSk } from './arb-scaffold-sk';

const loginURL = "https://" + window.location.host + "/loginstatus/";
fetchMock.get(loginURL, {
    "Email":"user@google.com",
    "LoginURL":"https://accounts.google.com/",
    "IsAGoogler":true,
});
fetchMock.get("https://skia.org/loginstatus/", {
    "Email":"user@google.com",
    "LoginURL":"https://accounts.google.com/",
    "IsAGoogler":true,
});

import './index';