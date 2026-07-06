package main

import (
	"fmt"
	"golang.org/x/net/publicsuffix"
)

func main() {
	// Try more dyndns variants
	domains := []string{
		"myhome.dyndns.org",
		"test.dyndns.org",
		"custom.dyndns.org",
		"blogspot.com",
		"myblog.blogspot.com",
		"example.heroku.com",
		"myapp.herokuapp.com",
		"myapp.cloudfunctions.net",
		"example.gitlab.io",
		"myproject.gitlab.io",
		"myapp.onrender.com",
		"example.r.appspot.com",
	}
	for _, d := range domains {
		tld, icann := publicsuffix.PublicSuffix(d)
		match := "✓"
		if tld == d {
			match = "✗ (tld==domain)"
		}
		if !icann && tld != d && tld != "" {
			match = "✓ GOOD private-suffix test"
		}
		fmt.Printf("%-35s → suffix=%-25s icann=%v  %s\n", d, tld, icann, match)
	}
}
