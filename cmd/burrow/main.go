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
	"github.com/janiskrasemann/burrow/internal/immo"
	"github.com/janiskrasemann/burrow/internal/mailer"
	"github.com/janiskrasemann/burrow/internal/renderer"
	"github.com/robfig/cron/v3"
)

//go:embed assets/header.jpg
var headerImage []byte

func main() {
	configPath := flag.String("config", "/etc/burrow/config.yaml", "path to config file")
	once := flag.Bool("once", false, "run once immediately and exit")
	test := flag.Bool("test", false, "render output and open HTML in browser instead of sending email")
	job := flag.String("job", "digest", "which job --once/--test refer to: digest|immo")
	flag.Parse()

	if *job != "digest" && *job != "immo" {
		log.Fatalf("Unknown job %q (want digest or immo)", *job)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *job == "immo" && cfg.Immo == nil {
		log.Fatal("Job immo requested but config has no immo section")
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
	for _, src := range cfg.Sources {
		switch src.Type {
		case "weather":
			fetchers = append(fetchers, fetcher.NewWeather(httpClient, src.Latitude, src.Longitude, src.Name))
		case "readwise":
			fetchers = append(fetchers, fetcher.NewReadwise(httpClient, src.APIToken))
		case "hackernews":
			fetchers = append(fetchers, fetcher.NewHackerNews(httpClient))
		case "reddit":
			subs := src.Subreddits
			if len(subs) == 0 && src.Subreddit != "" {
				subs = []string{src.Subreddit}
			}
			fetchers = append(fetchers, fetcher.NewReddit(httpClient, subs, src.Label))
		case "nitter":
			fetchers = append(fetchers, fetcher.NewNitter(httpClient, src.NitterInstance, src.Usernames, src.Limit))
		default:
			log.Fatalf("Unknown source type: %q", src.Type)
		}
	}

	agg := aggregator.New(fetchers...)

	mail := mailer.New(cfg.Email.From, cfg.Email.To, cfg.Email.ResendAPIKey, headerImage)

	// Immo setup (only when configured)
	var (
		immoScrapers []immo.Scraper
		immoHTMLTpl  string
		immoTextTpl  string
		immoCriteria immo.Criteria
	)
	if cfg.Immo != nil {
		for _, site := range cfg.Immo.Sites {
			switch site.Type {
			case "weserland":
				immoScrapers = append(immoScrapers, immo.NewWeserland(httpClient, site.URL))
			case "hapke":
				immoScrapers = append(immoScrapers, immo.NewHapke(httpClient, site.URL))
			default:
				log.Fatalf("Unknown immo site type: %q", site.Type)
			}
		}

		h, err := os.ReadFile("templates/immo.html")
		if err != nil {
			log.Fatalf("Failed to read immo HTML template: %v", err)
		}
		txt, err := os.ReadFile("templates/immo.txt")
		if err != nil {
			log.Fatalf("Failed to read immo text template: %v", err)
		}
		immoHTMLTpl, immoTextTpl = string(h), string(txt)

		immoCriteria = immo.Criteria{
			MinPriceCents: int64(cfg.Immo.Criteria.MinPrice) * 100,
			MaxPriceCents: int64(cfg.Immo.Criteria.MaxPrice) * 100,
			Locations:     cfg.Immo.Criteria.Locations,
		}
	}

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

	runImmo := func() {
		log.Println("Starting immo check...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		listings := immo.ScrapeAll(ctx, immoScrapers)
		filtered := immo.Filter(listings, immoCriteria)
		log.Printf("Immo: %d listings scraped, %d match criteria", len(listings), len(filtered))

		state, err := immo.LoadState(cfg.Immo.StatePath)
		if err != nil {
			log.Printf("Failed to load immo state: %v", err)
			return
		}

		diff := state.Diff(filtered, time.Now())

		if diff.Empty() {
			log.Println("Immo: no new listings or price drops")
			// Still save so last_seen stays current and old records get pruned.
			if err := state.Save(cfg.Immo.StatePath); err != nil {
				log.Printf("Failed to save immo state: %v", err)
			}
			return
		}

		html, text, err := immo.RenderEmail(immoHTMLTpl, immoTextTpl, immo.EmailData{
			Date:       time.Now().Format("January 2, 2006"),
			PriceDrops: diff.PriceDrops,
			New:        diff.New,
		})
		if err != nil {
			log.Printf("Failed to render immo email: %v", err)
			return
		}

		to := cfg.Immo.To
		if to == "" {
			to = cfg.Email.To
		}

		// State is saved only after a successful send — a failed send
		// retries on the next cron run.
		if err := mail.SendEmail(to, immo.Subject(diff), html, text); err != nil {
			log.Printf("Failed to send immo email: %v", err)
			return
		}
		if err := state.Save(cfg.Immo.StatePath); err != nil {
			log.Printf("Failed to save immo state: %v", err)
		}

		log.Printf("Immo email sent: %d new, %d price drops", len(diff.New), len(diff.PriceDrops))
	}

	if *test {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if *job == "immo" {
			// Test mode bypasses the state diff entirely and renders all
			// filtered listings (state is neither read nor written), so the
			// template and the live scrapers can be checked in isolation.
			log.Println("Test mode: rendering immo email and opening in browser...")
			listings := immo.ScrapeAll(ctx, immoScrapers)
			filtered := immo.Filter(listings, immoCriteria)
			log.Printf("Immo: %d listings scraped, %d match criteria", len(listings), len(filtered))

			html, _, err := immo.RenderEmail(immoHTMLTpl, immoTextTpl, immo.EmailData{
				Date: time.Now().Format("January 2, 2006"),
				New:  filtered,
			})
			if err != nil {
				log.Fatalf("Failed to render immo email: %v", err)
			}
			openInBrowser(html)
			return
		}

		log.Println("Test mode: rendering digest and opening in browser...")
		results := agg.FetchAll(ctx)

		email, err := rend.Render(results, cfg.Edition+1)
		if err != nil {
			log.Fatalf("Failed to render digest: %v", err)
		}
		openInBrowser(email.HTML)
		return
	}

	if *once {
		if *job == "immo" {
			runImmo()
		} else {
			runDigest()
		}
		return
	}

	c := cron.New()
	_, err = c.AddFunc(cfg.Schedule, runDigest)
	if err != nil {
		log.Fatalf("Failed to add cron schedule %q: %v", cfg.Schedule, err)
	}
	if cfg.Immo != nil {
		_, err = c.AddFunc(cfg.Immo.Schedule, runImmo)
		if err != nil {
			log.Fatalf("Failed to add immo cron schedule %q: %v", cfg.Immo.Schedule, err)
		}
	}
	c.Start()

	log.Printf("Burrow started. Schedule: %s", cfg.Schedule)
	if cfg.Immo != nil {
		log.Printf("Immo watch enabled. Schedule: %s", cfg.Immo.Schedule)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	c.Stop()
}

// openInBrowser writes the HTML to a temp file and opens it in the default
// browser (used by both --test paths).
func openInBrowser(html string) {
	f, err := os.CreateTemp("", "burrow-digest-*.html")
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(html); err != nil {
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
}
