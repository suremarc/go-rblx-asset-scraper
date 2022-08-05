package main

import "os"

func init() {
	key = os.Getenv("SPACES_KEY")
	if key == "" {
		panic("no key provided")
	}
	secret = os.Getenv("SPACES_SECRET")
	if secret == "" {
		panic("no secret provided")
	}
	bucket = os.Getenv("SPACES_BUCKET")
	if bucket == "" {
		panic("no bucket provided")
	}
	region = os.Getenv("SPACES_REGION")
	if region == "" {
		panic("no region provided")
	}
}
