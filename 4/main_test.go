package main

import (
	"context"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"testing"
)

func TestLinkParser(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	testFn := func(t *testing.T, filename string, expectedResult []Link) {
		tokens := streamHtmlTokens(ctx, filename)
		links := streamLinks(ctx, tokens)
		idx := 0

		for got := range links {
			expected := expectedResult[idx]

			if !reflect.DeepEqual(got, expected) {
				t.Errorf("Expected %v, got %v\n", expected, got)
				break
			}

			idx++
		}
	}

	t.Run("Extract ex1.html links", func(t *testing.T) {
		testFn(t, "ex1.html", []Link{{"/other-page", "A link to another page"}})
	})

	t.Run("Extract ex2.html links", func(t *testing.T) {
		testFn(t, "ex2.html", []Link{
			{Href: "https://www.twitter.com/joncalhoun", Text: "Check me out on twitter"},
			{Href: "https://github.com/gophercises", Text: "Gophercises is on Github!"},
		})
	})

	t.Run("Extract ex3.html links", func(t *testing.T) {
		testFn(t, "ex3.html", []Link{
			{Href: "#", Text: "Login"},
			{Href: "/lost", Text: "Lost? Need help?"},
			{Href: "https://twitter.com/marcusolsson", Text: "@marcusolsson"},
		})
	})

	t.Run("Extract ex4.html links", func(t *testing.T) {
		testFn(t, "ex4.html", []Link{{"/dog-cat", "dog cat"}})
	})

	t.Run("Extract ex5.html links", func(t *testing.T) {
		testFn(t, "ex5.html", []Link{
			{"/dog", "Something in a span Text not in a span Bold text!"},
			{"/dog/golden", "Something in a span Text not in a span Bold text!"},
		})
	})

	t.Run("Extract ex6.html links", func(t *testing.T) {
		testFn(t, "ex6.html", []Link{
			{"/dog", "nested dog link"},
			{"#", "Something here !"},
		})
	})

	t.Run("Extract ex7.html links", func(t *testing.T) {
		testFn(t, "ex7.html", []Link{
			{"/friend", "John Sullivan!!!!"},
			{"/inner", "dog strong my friend?"},
			{"/dog", "nested dog link ###"},
			{"/nan", "Otherwise"},
			{"#", "Something here ! $"},
		})
	})

	t.Run("Extract ex8.html links", func(t *testing.T) {
		testFn(t, "ex8.html", []Link{
			{"", "Empty href"},
			{"/href", "with href"},
		})
	})
}
