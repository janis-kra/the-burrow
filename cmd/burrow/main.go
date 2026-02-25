package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/janiskrasemann/burrow/internal/aggregator"
	"github.com/janiskrasemann/burrow/internal/config"
	"github.com/janiskrasemann/burrow/internal/fetcher"
	"github.com/janiskrasemann/burrow/internal/mailer"
	"github.com/janiskrasemann/burrow/internal/renderer"
	"github.com/robfig/cron/v3"
)

//go:embed assets/header.jpg
var headerImage []byte

func main() {
	configPath := flag.String("config", "/etc/burrow/config.yaml", "path to config file")
	once := flag.Bool("once", false, "run once immediately and exit")
	test := flag.Bool("test", false, "render digest and open HTML in browser instead of sending email")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	htmlTpl, err := os.ReadFile("templates/digest.html")
	if err != nil {
		log.Fatalf("Failed to read HTML template: %v", err)
	}
	textTpl, err := os.ReadFile("templates/digest.txt")
	if err != nil {
		log.Fatalf("Failed to read text template: %v", err)
	}

	rend, err := renderer.New(string(htmlTpl), string(textTpl))
	if err != nil {
		log.Fatalf("Failed to initialize renderer: %v", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var fetchers []fetcher.Fetcher
	var unsplashFetcher *fetcher.Unsplash
	for _, src := range cfg.Sources {
		switch src.Type {
		case "weather":
			fetchers = append(fetchers, fetcher.NewWeather(httpClient, src.Latitude, src.Longitude, src.Name))
		case "readwise":
			fetchers = append(fetchers, fetcher.NewReadwise(httpClient, src.APIToken))
		case "hackernews":
			fetchers = append(fetchers, fetcher.NewHackerNews(httpClient))
		case "reddit":
			fetchers = append(fetchers, fetcher.NewReddit(src.Subreddit))
		case "nitter":
			fetchers = append(fetchers, fetcher.NewNitter(httpClient, src.NitterInstance, src.Usernames, src.Limit))
		case "unsplash":
			unsplashFetcher = fetcher.NewUnsplash(httpClient, src.APIToken, src.Query)
		default:
			log.Fatalf("Unknown source type: %q", src.Type)
		}
	}

	agg := aggregator.New(fetchers...)

	mail := mailer.New(cfg.Email.From, cfg.Email.To, cfg.Email.ResendAPIKey, headerImage)

	runDigest := func() {
		log.Println("Starting digest generation...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Reload config to get current edition number
		latestCfg, err := config.Load(*configPath)
		if err != nil {
			log.Printf("Failed to reload config: %v", err)
			return
		}
		edition := latestCfg.Edition + 1

		results := agg.FetchAll(ctx)
		results = fetchUnsplash(ctx, unsplashFetcher, results)

		email, err := rend.Render(results, edition)
		if err != nil {
			log.Printf("Failed to render digest: %v", err)
			return
		}

		if err := mail.Send(email); err != nil {
			log.Printf("Failed to send digest: %v", err)
			return
		}

		if err := config.IncrementEdition(*configPath); err != nil {
			log.Printf("Failed to update edition counter: %v", err)
		}

		log.Printf("Digest #%d sent successfully!", edition)
	}

	if *test {
		log.Println("Test mode: rendering digest and opening in browser...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		results := agg.FetchAll(ctx)
		results = fetchUnsplash(ctx, unsplashFetcher, results)

		email, err := rend.Render(results, cfg.Edition+1)
		if err != nil {
			log.Fatalf("Failed to render digest: %v", err)
		}

		f, err := os.CreateTemp("", "burrow-digest-*.html")
		if err != nil {
			log.Fatalf("Failed to create temp file: %v", err)
		}
		if _, err := f.WriteString(email.HTML); err != nil {
			f.Close()
			log.Fatalf("Failed to write HTML: %v", err)
		}
		f.Close()

		log.Printf("HTML written to %s", f.Name())

		var cmd string
		switch runtime.GOOS {
		case "darwin":
			cmd = "open"
		case "linux":
			cmd = "xdg-open"
		default:
			cmd = "open"
		}
		if err := exec.Command(cmd, f.Name()).Start(); err != nil {
			log.Printf("Failed to open browser: %v", err)
		}
		return
	}

	if *once {
		runDigest()
		return
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.Schedule, runDigest)
	if err != nil {
		log.Fatalf("Failed to add cron schedule %q: %v", cfg.Schedule, err)
	}
	c.Start()

	log.Printf("Burrow started. Schedule: %s", cfg.Schedule)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	c.Stop()
}

// fetchUnsplash runs the Unsplash fetcher after the aggregator, using the
// Readwise highlight's BookTitle as the topic query for contextual imagery.
func fetchUnsplash(ctx context.Context, uf *fetcher.Unsplash, results []fetcher.Result) []fetcher.Result {
	if uf == nil {
		return results
	}

	// Extract BookTitle from Readwise results for topic-aware image search
	for _, r := range results {
		if r.Name == "Readwise" && r.Error == nil {
			if highlights, ok := r.Data.([]fetcher.Highlight); ok && len(highlights) > 0 {
				if highlights[0].BookTitle != "" {
					uf.SetTopicQuery(highlights[0].BookTitle)
				}
			}
		}
	}

	data, err := uf.Fetch(ctx)
	results = append(results, fetcher.Result{
		Name:  uf.Name(),
		Data:  data,
		Error: err,
	})
	if err != nil {
		log.Printf("Unsplash fetch failed: %v", err)
	}
	return results
}
