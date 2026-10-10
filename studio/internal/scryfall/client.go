// Package scryfall is a small, polite client for https://api.scryfall.com (identifying headers, request spacing,
// 429/5xx backoff). Replaces scryfall-importer/scryfall.
package scryfall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	apiBase   = "https://api.scryfall.com"
	userAgent = "TCGStudio/0.1 (TCG Card Shop Simulator custom cards mod)"
	minGap    = 110 * time.Millisecond // Scryfall asks for 50–100 ms between requests
)

type Client struct {
	http        *http.Client
	mu          sync.Mutex
	last        time.Time
	unthrottled bool
}

func New() *Client { return &Client{http: &http.Client{Timeout: 60 * time.Second}} }

// NewUnthrottled is for card images on cards.scryfall.io (a CDN without the API's rate limit); retries still apply.
func NewUnthrottled() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}, unthrottled: true}
}

type Set struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	SetType    string `json:"set_type"`
	ReleasedAt string `json:"released_at"`
	CardCount  int    `json:"card_count"`
	Digital    bool   `json:"digital"`
	IconSVGURI string `json:"icon_svg_uri"`
	ParentCode string `json:"parent_set_code"`
}

type ImageURIs struct {
	Small  string `json:"small"`
	Normal string `json:"normal"`
	Large  string `json:"large"`
	PNG    string `json:"png"`
}

type Face struct {
	Name       string     `json:"name"`
	OracleText string     `json:"oracle_text"`
	FlavorText string     `json:"flavor_text"`
	Artist     string     `json:"artist"`
	ImageURIs  *ImageURIs `json:"image_uris"`
	TypeLine   string     `json:"type_line"`
	ManaCost   string     `json:"mana_cost"`
	Power      string     `json:"power"`
	Toughness  string     `json:"toughness"`
	Colors     []string   `json:"colors"`
}

type Prices struct {
	USD     *string `json:"usd"`
	USDFoil *string `json:"usd_foil"`
	USDEtch *string `json:"usd_etched"`
	EUR     *string `json:"eur"`
	EURFoil *string `json:"eur_foil"`
}

type Card struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Lang            string     `json:"lang"`
	Layout          string     `json:"layout"`
	CollectorNumber string     `json:"collector_number"`
	Rarity          string     `json:"rarity"`
	OracleText      string     `json:"oracle_text"`
	FlavorText      string     `json:"flavor_text"`
	Artist          string     `json:"artist"`
	TypeLine        string     `json:"type_line"`
	ManaCost        string     `json:"mana_cost"`
	CMC             float64    `json:"cmc"`
	Power           string     `json:"power"`
	Toughness       string     `json:"toughness"`
	Colors          []string   `json:"colors"`
	ColorIdentity   []string   `json:"color_identity"`
	ImageURIs       *ImageURIs `json:"image_uris"`
	CardFaces       []Face     `json:"card_faces"`
	Prices          Prices     `json:"prices"`
	Finishes        []string   `json:"finishes"`
	Variation       bool       `json:"variation"`
	Digital         bool       `json:"digital"`
	FullArt         bool       `json:"full_art"`
	Promo           bool       `json:"promo"`
	BorderColor     string     `json:"border_color"`
	FrameEffects    []string   `json:"frame_effects"`
}

// FrontImage returns the best image URIs for the card's front face.
func (c *Card) FrontImage() *ImageURIs {
	if c.ImageURIs != nil {
		return c.ImageURIs
	}
	if len(c.CardFaces) > 0 && c.CardFaces[0].ImageURIs != nil {
		return c.CardFaces[0].ImageURIs
	}
	return nil
}

// BackImage returns the back face's image URIs for double-faced cards (transform, modal DFC, …: each face has its own
// picture and the card has no top-level image), or nil. Split / adventure / flip cards are one picture: nil.
func (c *Card) BackImage() *ImageURIs {
	if c.ImageURIs != nil || len(c.CardFaces) < 2 {
		return nil
	}
	return c.CardFaces[1].ImageURIs
}

func (c *Card) Text() string {
	if c.OracleText != "" || len(c.CardFaces) == 0 {
		return c.OracleText
	}
	t := ""
	for i, f := range c.CardFaces {
		if i > 0 {
			t += "\n//\n"
		}
		t += f.Name + ": " + f.OracleText
	}
	return t
}

func (c *Card) ArtistName() string {
	if c.Artist != "" || len(c.CardFaces) == 0 {
		return c.Artist
	}
	return c.CardFaces[0].Artist
}

func (c *Card) ColorList() []string {
	if len(c.Colors) > 0 || len(c.CardFaces) == 0 {
		return c.Colors
	}
	return c.CardFaces[0].Colors
}

func ParsePrice(p *string) *float64 {
	if p == nil || *p == "" {
		return nil
	}
	v, err := strconv.ParseFloat(*p, 64)
	if err != nil {
		return nil
	}
	return &v
}

type list[T any] struct {
	Data     []T    `json:"data"`
	HasMore  bool   `json:"has_more"`
	NextPage string `json:"next_page"`
	Total    int    `json:"total_cards"`
}

type apiError struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Details string `json:"details"`
}

func (c *Client) Sets(ctx context.Context) ([]Set, error) {
	var l list[Set]
	if err := c.getJSON(ctx, apiBase+"/sets", &l); err != nil {
		return nil, err
	}
	return l.Data, nil
}

func (c *Client) GetSet(ctx context.Context, code string) (*Set, error) {
	var s Set
	if err := c.getJSON(ctx, apiBase+"/sets/"+url.PathEscape(code), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SetCards returns every printing in a set (unique=prints includes variants/showcase versions), in set order.
func (c *Client) SetCards(ctx context.Context, code string, progress func(got, total int)) ([]Card, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("e:%s", code))
	q.Set("unique", "prints")
	q.Set("order", "set")
	q.Set("include_extras", "true")
	q.Set("include_variations", "true")
	next := apiBase + "/cards/search?" + q.Encode()
	var all []Card
	for next != "" {
		var page list[Card]
		if err := c.getJSON(ctx, next, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if progress != nil {
			progress(len(all), page.Total)
		}
		next = ""
		if page.HasMore {
			next = page.NextPage
		}
	}
	return all, nil
}

// Download fetches a file (images, SVG icons) with the same politeness rules.
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
		if json.Unmarshal(body, &e) == nil && e.Details != "" {
			return nil, fmt.Errorf("scryfall: %s", e.Details)
		}
		return nil, fmt.Errorf("scryfall: HTTP %d for %s", resp.StatusCode, u)
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
