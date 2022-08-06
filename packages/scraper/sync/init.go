package main

import "os"

func init() {
	key = os.Getenv("WASABI_ACCESS_KEY")
	if key == "" {
		panic("no key provided")
	}
	secret = os.Getenv("WASABI_SECRET_KEY")
	if secret == "" {
		panic("no secret provided")
	}
	bucket = os.Getenv("WASABI_BUCKET")
	if bucket == "" {
		panic("no bucket provided")
	}
	region = os.Getenv("WASABI_REGION")
	if region == "" {
		panic("no region provided")
	}
}
