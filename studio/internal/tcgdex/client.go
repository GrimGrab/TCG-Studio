// Package tcgdex is a small, polite client for the Pokémon TCG database at https://api.tcgdex.net (v2 REST API):
// sets, series and cards with TCGplayer (USD) and Cardmarket (EUR) prices. Card images come from assets.tcgdex.net.
package tcgdex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiBase   = "https://api.tcgdex.net/v2/"
	userAgent = "TCGStudio/0.1 (TCG Card Shop Simulator custom cards mod)"
	minGap    = 50 * time.Millisecond // no published limit; stay gentle
)

// Languages TCGdex serves card data in (set lists and names differ per language).
var Languages = []string{"en", "fr", "de", "it", "es", "pt", "ja"}

type Client struct {
	http        *http.Client
	mu          sync.Mutex
	last        time.Time
	unthrottled bool
}

func New() *Client { return &Client{http: &http.Client{Timeout: 60 * time.Second}} }

// NewUnthrottled is for images on the asset CDN; retries still apply.
func NewUnthrottled() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}, unthrottled: true}
}

type CardCount struct {
	Total    int `json:"total"`
	Official int `json:"official"`
}

// SetBrief is a set as listed by /sets and inside /series/{id}.
type SetBrief struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Logo      string    `json:"logo"`
	Symbol    string    `json:"symbol"`
	CardCount CardCount `json:"cardCount"`
}

type SerieBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Serie struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	ReleaseDate string     `json:"releaseDate"`
	Sets        []SetBrief `json:"sets"`
}

type CardBrief struct {
	ID      string `json:"id"`
	LocalID string `json:"localId"`
	Name    string `json:"name"`
	Image   string `json:"image"`
}

type Set struct {
	SetBrief
	ReleaseDate string      `json:"releaseDate"`
	Serie       SerieBrief  `json:"serie"`
	Cards       []CardBrief `json:"cards"`
}

type Attack struct {
	Cost   []string `json:"cost"`
	Name   string   `json:"name"`
	Effect string   `json:"effect"`
	Damage any      `json:"damage"` // number or string ("60+")
}

type Ability struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Effect string `json:"effect"`
}

type Variants struct {
	Normal       bool `json:"normal"`
	Reverse      bool `json:"reverse"`
	Holo         bool `json:"holo"`
	FirstEdition bool `json:"firstEdition"`
}

type TCGPlayerPrice struct {
	Low    *float64 `json:"lowPrice"`
	Mid    *float64 `json:"midPrice"`
	Market *float64 `json:"marketPrice"`
}

// Pricing: tcgplayer (USD) has one entry per variant key ("normal", "holofoil", "reverse-holofoil", "1st-edition-holofoil", …)
// next to "unit"/"updated"; cardmarket (EUR) has flat fields, "-holo" ones for the reverse/holo version.
type Pricing struct {
	Cardmarket map[string]any             `json:"cardmarket"`
	TCGPlayer  map[string]json.RawMessage `json:"tcgplayer"`
}

type Card struct {
	ID          string    `json:"id"`
	LocalID     string    `json:"localId"`
	Name        string    `json:"name"`
	Category    string    `json:"category"` // Pokemon, Trainer, Energy
	Illustrator string    `json:"illustrator"`
	Image       string    `json:"image"`
	Rarity      string    `json:"rarity"`
	HP          int       `json:"hp"`
	Types       []string  `json:"types"`
	Stage       string    `json:"stage"`
	Suffix      string    `json:"suffix"`
	EvolveFrom  string    `json:"evolveFrom"`
	Description string    `json:"description"` // flavour text
	Effect      string    `json:"effect"`      // trainer/energy text
	TrainerType string    `json:"trainerType"`
	EnergyType  string    `json:"energyType"`
	Abilities   []Ability `json:"abilities"`
	Attacks     []Attack  `json:"attacks"`
	Variants    Variants  `json:"variants"`
	Pricing     *Pricing  `json:"pricing"`
}

// ImageURL returns the card image in PNG ("high" = 600×825, "low" = 245×337). Empty when the card has no image.
func (c *Card) ImageURL(quality string) string {
	if c.Image == "" {
		return ""
	}
	return c.Image + "/" + quality + ".png"
}

// TypeLine reads like "Pokémon — Fire Stage2" / "Trainer — Item" / "Energy — Basic".
func (c *Card) TypeLine() string {
	var parts []string
	switch c.Category {
	case "Pokemon":
		parts = append(parts, c.Types...)
		if c.Stage != "" {
			parts = append(parts, c.Stage)
		}
		if c.Suffix != "" {
			parts = append(parts, c.Suffix)
		}
	case "Trainer":
		parts = append(parts, c.TrainerType)
	case "Energy":
		parts = append(parts, c.EnergyType)
	}
	cat := c.Category
	if cat == "Pokemon" {
		cat = "Pokémon"
	}
	s := strings.TrimSpace(strings.Join(parts, " "))
	if s == "" {
		return cat
	}
	return cat + " — " + s
}

// Text is the card's rules text: abilities, attacks (with damage) or the trainer/energy effect.
func (c *Card) Text() string {
	var lines []string
	for _, a := range c.Abilities {
		lines = append(lines, fmt.Sprintf("%s: %s — %s", a.Type, a.Name, a.Effect))
	}
	for _, a := range c.Attacks {
		l := a.Name
		if d := damage(a.Damage); d != "" {
			l += " " + d
		}
		if a.Effect != "" {
			l += " — " + a.Effect
		}
		lines = append(lines, l)
	}
	if c.Effect != "" {
		lines = append(lines, c.Effect)
	}
	return strings.Join(lines, "\n")
}

func damage(v any) string {
	switch d := v.(type) {
	case float64:
		return strconv.FormatFloat(d, 'f', -1, 64)
	case string:
		return d
	}
	return ""
}

// Prices picks the card's real prices: usd = TCGplayer market price of the base version (normal, else holofoil, else
// any other version); usdFoil = the reverse holo's market price, or the base scaled by Cardmarket's holo/normal trend
// ratio when only Cardmarket knows the reverse; eur = the Cardmarket trend (avg as fallback). Any may be nil.
func (c *Card) Prices() (usd, usdFoil, eur *float64) {
	if c.Pricing == nil {
		return nil, nil, nil
	}
	tp := map[string]TCGPlayerPrice{}
	for k, raw := range c.Pricing.TCGPlayer {
		var p TCGPlayerPrice
		if k != "unit" && k != "updated" && json.Unmarshal(raw, &p) == nil {
			tp[k] = p
		}
	}
	market := func(k string) *float64 {
		p, ok := tp[k]
		if !ok {
			return nil
		}
		for _, v := range []*float64{p.Market, p.Mid, p.Low} {
			if v != nil && *v > 0 {
				return v
			}
		}
		return nil
	}
	for _, k := range []string{"normal", "holofoil", "unlimited", "unlimited-holofoil", "1st-edition", "1st-edition-holofoil"} {
		if usd = market(k); usd != nil {
			break
		}
	}
	if usd == nil {
		for k := range tp {
			if k != "reverse-holofoil" {
				if usd = market(k); usd != nil {
					break
				}
			}
		}
	}
	usdFoil = market("reverse-holofoil")

	cm := func(k string) *float64 {
		if f, ok := c.Pricing.Cardmarket[k].(float64); ok && f > 0 {
			return &f
		}
		return nil
	}
	if eur = cm("trend"); eur == nil {
		eur = cm("avg")
	}
	if usdFoil == nil && usd != nil && c.Variants.Reverse && eur != nil {
		if h := cm("trend-holo"); h != nil {
			f := *usd * *h / *eur
			usdFoil = &f
		}
	}
	return usd, usdFoil, eur
}

type apiError struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func (c *Client) Sets(ctx context.Context, lang string) ([]SetBrief, error) {
	var out []SetBrief
	return out, c.getJSON(ctx, apiBase+lang+"/sets", &out)
}

func (c *Client) Series(ctx context.Context, lang string) ([]SerieBrief, error) {
	var out []SerieBrief
	return out, c.getJSON(ctx, apiBase+lang+"/series", &out)
}

func (c *Client) GetSerie(ctx context.Context, lang, id string) (*Serie, error) {
	var s Serie
	return &s, c.getJSON(ctx, apiBase+lang+"/series/"+url.PathEscape(id), &s)
}

func (c *Client) GetSet(ctx context.Context, lang, id string) (*Set, error) {
	var s Set
	return &s, c.getJSON(ctx, apiBase+lang+"/sets/"+url.PathEscape(id), &s)
}

func (c *Client) GetCard(ctx context.Context, lang, id string) (*Card, error) {
	var cd Card
	return &cd, c.getJSON(ctx, apiBase+lang+"/cards/"+url.PathEscape(id), &cd)
}

// Download fetches a file (card images, logos) with the same retry rules.
func (c *Client) Download(ctx context.Context, u string) ([]byte, error) {
	resp, err := c.do(ctx, u, "*/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	resp, err := c.do(ctx, u, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) do(ctx context.Context, u, accept string) (*http.Response, error) {
	backoff := time.Second
	for attempt := 0; ; attempt++ {
		c.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", accept)
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < 4 && ctx.Err() == nil {
				sleep(ctx, backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < 5 {
			wait := backoff
			if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && ra > 0 {
				wait = time.Duration(ra) * time.Second
			}
			sleep(ctx, wait)
			backoff *= 2
			continue
		}
		var e apiError
		if json.Unmarshal(body, &e) == nil && (e.Detail != "" || e.Title != "") {
			return nil, fmt.Errorf("tcgdex: %s %s", e.Title, e.Detail)
		}
		return nil, fmt.Errorf("tcgdex: HTTP %d for %s", resp.StatusCode, u)
	}
}

func (c *Client) wait() {
	if c.unthrottled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := minGap - time.Since(c.last); d > 0 {
		time.Sleep(d)
	}
	c.last = time.Now()
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
