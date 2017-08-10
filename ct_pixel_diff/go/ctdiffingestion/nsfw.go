package ctdiffingestion

import "strings"

const (
	HTTP  = "http://"
	HTTPS = "https://"
	WWW   = "www."
)

var (
	nsfwUrlsSlice = []string{
		"xhamster.com",
		"xvideos.com",
		"livejasmin.com",
		"pornhub.com",
		"redtube.com",
		"youporn.com",
		"xnxx.com",
		"tube8.com",
		"youjizz.com",
		"adultfriendfinder.com",
		"hardsextube.com",
		"yourlust.com",
		"drtuber.com",
		"beeg.com",
		"largeporntube.com",
		"nuvid.com",
		"bravotube.net",
		"spankwire.com",
		"discreethearts.com",
		"keezmovies.com",
		"xtube.com",
		"alphaporno.com",
		"4tube.com",
		"nudevista.com",
		"porntube.com",
		"xhamstercams.com",
		"porn.com",
		"video-one.com",
		"perfectgirls.net",
		"slutload.com",
		"sunporno.com",
		"tnaflix.com",
		"pornerbros.com",
		"h2porn.com",
		"adult-empire.com",
		"pornhublive.com",
		"sexitnow.com",
		"pornsharia.com",
		"freeones.com",
		"tubegalore.com",
		"xvideos.jp",
		"brazzers.com",
		"fapdu.com",
		"pornoxo.com",
		"extremetube.com",
		"hot-sex-tube.com",
		"xhamsterhq.com",
		"18andabused.com",
		"tubepleasure.com",
		"18schoolgirlz.com",
		"chaturbate.com",
		"motherless.com",
		"yobt.com",
		"empflix.com",
		"hellporno.com",
		"ashemaletube.com",
		"watchmygf.com",
		"redtubelive.com",
		"met-art.com",
		"gonzoxxxmovies.com",
		"shufuni.com",
		"vid2c.com",
		"dojki.com",
		"cerdas.com",
		"overthumbs.com",
		"xvideoslive.com",
		"playboy.com",
		"caribbeancom.com",
		"tubewolf.com",
		"xmatch.com",
		"ixxx.com",
		"nymphdate.com",
	}
	nsfwUrlsMap map[string]struct{} = nil
)

// Initialize a map of NSFW URLs to empty structs so that checking for
// membership can be done efficiently.
func init() {
	nsfwUrlsMap = make(map[string]struct{}, len(nsfwUrlsSlice))
	for _, url := range nsfwUrlsSlice {
		nsfwUrlsMap[url] = struct{}{}
	}
}

// Returns true if the given url is in the NSFW list and false otherwise.
func isNsfwUrl(url string) bool {
	formattedUrl := url

	// Strip either "http://" or "https://" prefix if it exists
	if strings.HasPrefix(formattedUrl, HTTP) {
		formattedUrl = strings.TrimPrefix(formattedUrl, HTTP)
	} else if strings.HasPrefix(formattedUrl, HTTPS) {
		formattedUrl = strings.TrimPrefix(formattedUrl, HTTPS)
	}

	// Strip "www." prefix if it exists
	if strings.HasPrefix(formattedUrl, WWW) {
		formattedUrl = strings.TrimPrefix(formattedUrl, WWW)
	}

	_, ok := nsfwUrlsMap[formattedUrl]
	return ok
}
