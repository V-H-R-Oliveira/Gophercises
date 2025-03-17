package sitemap

import (
	"context"
	"log"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Link struct {
	Href string
	Text string
}

func getTagAttributes(ctx context.Context, stopStream <-chan struct{}, token html.Token) <-chan html.Attribute {
	attributes := make(chan html.Attribute)

	go func() {
		defer close(attributes)

		for _, attribute := range token.Attr {
			select {
			case <-ctx.Done():
				return
			case <-stopStream:
				return
			case attributes <- attribute:
			}
		}
	}()

	return attributes
}

func cleanText(text string) string {
	cleanParts := []string{}

	for _, part := range strings.Split(text, " ") {
		withoutSpaces := strings.TrimSpace(part)

		if len(withoutSpaces) > 0 {
			cleanParts = append(cleanParts, withoutSpaces)
		}
	}

	return strings.Join(cleanParts, " ")
}

func StreamLinks(ctx context.Context, tokens <-chan html.Token) <-chan Link {
	links := make(chan Link)

	go func() {
		defer close(links)

		anchorStack, textStack := []string{}, []string{}
		current := -1

		parseToken := func(token html.Token) {
			switch token.Type {
			case html.StartTagToken:
				if token.DataAtom == atom.A {
					stopStream := make(chan struct{})

					for attribute := range getTagAttributes(ctx, stopStream, token) {
						if attribute.Key == "href" {
							anchorStack = append(anchorStack, attribute.Val)
							close(stopStream)
						}
					}

					textStack = append(textStack, "")
					current++
				}
			case html.EndTagToken:
				if token.DataAtom == atom.A {
					if len(anchorStack) == 0 {
						if current == 0 {
							textStack = textStack[current+1:]
						} else {
							textStack = textStack[:current]
						}

						current--
						return
					}

					href := anchorStack[current]
					anchorText := textStack[current]

					text := cleanText(anchorText)
					link := Link{Href: href, Text: text}
					links <- link

					anchorStack = anchorStack[:current]
					textStack = textStack[:current]
					current--
				}
			case html.TextToken:
				if len(textStack) > 0 {
					textStack[current] += token.Data
				}
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Println("[parseTokens] Parent cancelled the context")
				return
			case token, open := <-tokens:
				if !open {
					return
				}

				parseToken(token)
			}
		}
	}()

	return links
}
