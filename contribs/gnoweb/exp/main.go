package main

import (
	"fmt"
	"os"

	"golang.org/x/net/html"
)

func main() {
	n, err := html.Parse(os.Stdin)
	if err != nil {
		panic(err)
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		// if n.Type == html.ElementNode && n.Data == "a" {
		// 	// Do something with n...
		// }

		fmt.Printf("attr: %+v\n", n.Data)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}
