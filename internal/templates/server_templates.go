package templates

func DefaultIndexHTML() string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>Status</title></head><body><h1>Service Status</h1><p>OK</p></body></html>\n"
}

func DefaultAboutHTML() string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>About</title></head><body><h1>About</h1><p>Private service endpoint.</p></body></html>\n"
}

func DefaultRobotsTXT() string {
	return "User-agent: *\nDisallow: /api/\n"
}
