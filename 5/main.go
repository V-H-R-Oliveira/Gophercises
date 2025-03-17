package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	sitemap "sitemap/m/linkParser"
	"sync"
	"syscall"

	"golang.org/x/net/html"
)

func streamHtmlTokens(ctx context.Context, url string) <-chan html.Token {
	tokens := make(chan html.Token)

	go func() {
		defer close(tokens)

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		if err != nil {
			log.Printf("Failed to prepare request for url %s due error %v\n", url, err)
			return
		}

		response, err := http.DefaultClient.Do(request)

		if err != nil {
			log.Printf("Failed to fetch %s due error: %v\n", url, err)
			return
		}

		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			log.Printf("Failed to fetch %s due %s\n", url, response.Status)
			return
		}

		tokenizer := html.NewTokenizer(response.Body)

		for {
			tokenType := tokenizer.Next()

			if tokenType == html.ErrorToken {
				return
			}

			token := tokenizer.Token()

			select {
			case <-ctx.Done():
				log.Printf("Stop %s stream", url)
				return
			case tokens <- token:
			}
		}

	}()

	return tokens
}

func streamUrls(ctx context.Context, entrypoint string, baseUrl *url.URL) <-chan string {
	urls := make(chan string)

	go func() {
		defer close(urls)
		tokens := streamHtmlTokens(ctx, entrypoint)

		for link := range sitemap.StreamLinks(ctx, tokens) {
			parsedUrl, err := baseUrl.Parse(link.Href)

			if err != nil {
				log.Printf("Failed to parse url %s due error %v\n", link.Href, err)
				continue
			}

			if parsedUrl.Hostname() == baseUrl.Hostname() {
				urls <- parsedUrl.String()
			}
		}
	}()

	return urls
}

func generateSitemapFile(filename string, data map[string]bool) {
	fmt.Println("Writing sitemap to file", filename)

	fileHandler, err := os.Create(filename)

	if err != nil {
		log.Fatal(err)
	}

	defer fileHandler.Close()

	fmt.Fprintln(fileHandler, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	fmt.Fprintln(fileHandler, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")

	for url := range data {
		fmt.Fprintf(fileHandler, "\t<url>\n\t\t<loc>%s</loc>\n\t</url>\n", url)
	}

	fmt.Fprintln(fileHandler, "</urlset>")
}

func main() {
	entryPoint := flag.String("url", "https://books.toscrape.com/", "entrypoint url")
	workers := flag.Int("workers", 4, "workers amount")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	entryPointUrl, err := url.Parse(*entryPoint)

	if err != nil {
		log.Fatalf("Failed to parse %s due error %s\n", entryPointUrl, err)
	}

	seen := map[string]bool{}
	queue := make([]string, *workers, 10_000)
	queue[0] = *entryPoint

	var wg sync.WaitGroup

	crawler := func(url string, output chan<- string) {
		defer wg.Done()

		for newUrl := range streamUrls(ctx, url, entryPointUrl) {
			output <- newUrl
		}
	}

	for len(queue) > 0 {
		batch := queue[:*workers]
		newUrls := make(chan string)

		for _, url := range batch {
			_, has := seen[url]

			if url == "" || has {
				continue
			}

			fmt.Println("Visit", url)

			seen[url] = true

			wg.Add(1)
			go crawler(url, newUrls)
		}

		go func() {
			wg.Wait()
			close(newUrls)
		}()

		for url := range newUrls {
			queue = append(queue, url)
		}

		if len(queue) >= *workers {
			queue = queue[*workers:]
		} else {
			queue = queue[1:]
		}
	}

	generateSitemapFile("test.xml", seen)
}
