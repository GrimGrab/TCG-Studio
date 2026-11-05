package customers

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestDefaults(t *testing.T) {
	d, err := LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Traits) == 0 || len(d.Situations) == 0 {
		t.Fatalf("defaults: %d traits, %d situations", len(d.Traits), len(d.Situations))
	}
	situations := map[string]bool{}
	for _, s := range d.Situations {
		situations[s.ID] = true
	}
	for _, p := range d.EmployeePerks {
		if p.Description == "" {
			t.Errorf("employee perk %s has no description: the whiteboard's guide shows it", p.ID)
		}
	}
	ids := d.TraitIDs()
	for _, tr := range d.Traits {
		if tr.Description == "" {
			t.Errorf("trait %s has no description: the whiteboard's guide shows it", tr.ID)
		}
		if tr.Opposite != "" && !ids[tr.Opposite] {
			t.Errorf("trait %s: unknown opposite %q", tr.ID, tr.Opposite)
		}
		for s := range tr.Lines {
			if !situations[s] {
				t.Errorf("trait %s: lines for unknown situation %q", tr.ID, s)
			}
		}
	}
	for s := range d.Lines {
		if !situations[s] {
			t.Errorf("lines for unknown situation %q", s)
		}
	}
	for _, s := range d.Situations {
		if len(d.Lines[s.ID]) == 0 {
			t.Errorf("situation %s has no built-in lines", s.ID)
		}
	}
}

// Every situation the mod says something in is listed (Dialogue.Say(c, "<id>", …) in Runtime/Customers).
func TestModSituationsListed(t *testing.T) {
	d, _ := LoadDefaults()
	have := map[string]bool{}
	for _, s := range d.Situations {
		have[s.ID] = true
	}
	dir := filepath.Join("..", "..", "..", "src", "TCGCustomCards", "Runtime", "Customers")
	files, _ := filepath.Glob(filepath.Join(dir, "*.cs"))
	if len(files) == 0 {
		t.Skip("mod source not found")
	}
	// Literal situations, also the ones chosen inline (cond ? "a" : "b").
	say := regexp.MustCompile(`\bSay\([^,]+,\s*([^,]+),`)
	word := regexp.MustCompile(`"([A-Za-z]+)"`)
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for _, m := range say.FindAllSubmatch(b, -1) {
			for _, w := range word.FindAllSubmatch(m[1], -1) {
				if !have[string(w[1])] {
					t.Errorf("%s says %q, which defaults.json doesn't list", filepath.Base(f), w[1])
				}
			}
		}
	}
}

func TestLinesRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "game")
	in := SetupLines{Lines: map[string][]string{"enter": {" Hi! ", ""}, "tip": {""}}, Removed: map[string][]string{"enter": {"Hello!"}}}
	if err := SaveLines(dir, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLines(dir)
	if err != nil || len(got.Lines) != 1 || len(got.Lines["enter"]) != 1 || got.Lines["enter"][0] != "Hi!" || got.Removed["enter"][0] != "Hello!" {
		t.Fatalf("lines: %v %v", got, err)
	}
	if err := SaveLines(dir, SetupLines{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, LinesFile)); !os.IsNotExist(err) {
		t.Fatal("empty lines should remove the file")
	}
}

func TestFromSaves(t *testing.T) {
	dir := t.TempDir()
	// As the mod writes it: compact, gzip, -1 fields left out.
	doc := `{"personas":{"regulars":[` +
		`{"id":"c-1","name":"Maya R.","female":true,"model":3,"traits":["collector"],"regular":true,"origin":"promoted","visits":4,"lastDay":12,"spent":51.5},` +
		`{"id":"c-2","name":"Leo K.","traits":[],"regular":true,"origin":"promoted"},` +
		`{"id":"a-1","name":"Made","origin":"authored"}]}}`
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(doc))
	_ = w.Close()
	if err := os.WriteFile(filepath.Join(dir, "slot1.json"), gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "slot2.json"), []byte("not json"), 0o644)
	got := FromSaves(dir, map[string]bool{})
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	m := got[0]
	if m.ID != "c-1" || !m.Female || m.Model != 3 || m.Visits != 4 || m.LastDay != 12 || m.Slot != "slot1" {
		t.Fatalf("maya: %+v", m)
	}
	l := got[1]
	if l.Model != -1 || l.Skill != -1 || l.LastDay != -1 {
		t.Fatalf("leo defaults: %+v", l)
	}
	if got := FromSaves(dir, map[string]bool{"c-1": true}); len(got) != 1 {
		t.Fatalf("customers already on the page are left out: %+v", got)
	}
}

func TestValidateCustomers(t *testing.T) {
	l := &setfmt.AccessoryLibrary{Customers: []setfmt.Customer{
		setfmt.NewCustomer("a", "Ann"),
		{ID: "a", Name: "Bob", Model: 99, Skill: 101, Traits: []string{"nope"}},
		{ID: "", Name: "x"},
	}}
	errs, warns := l.ValidateCustomers(map[string]bool{"collector": true})
	if len(errs) != 4 || len(warns) != 1 {
		t.Fatalf("errs %v warns %v", errs, warns)
	}
}

// lineTokens: the tokens the mod fills for each situation (Runtime/Customers: the Dialogue.Tokens each Say passes); {shop}
// works everywhere. A line using another token shows its fallback ("that card", "someone"), so built-in lines must not.
var lineTokens = map[string]string{
	"cheap": "price card", "pricey": "price card", "wishAsk": "card", "loyaltyUp": "nick", "loyaltyDown": "nick", "showOffCard": "name card price", "showOffReact": "name card price", "sellOffer": "card", "wishFound": "card price", "ripHit": "card price",
	"nearbyHit": "card name price", "nearbyHuge": "card name price", "gawk": "card name price", "rivalMeet": "rival",
	"cheerKick": "name", "booKick": "name", "cheerCatch": "name", "chatOpen": "name", "chatReply": "name", "chatSmall": "name",
	"chatSmallReply": "name", "chatAsk": "name", "chatAskReply": "name", "chatBye": "name", "chatReact": "about when", "tradeDone": "card price name",
	"tradeHustle": "card price name", "tradeRefuse": "name", "tradeRipped": "about card card2 price", "seeHustler": "about",
	"gossipPull": "about rival card card2 price when", "gossipTheft": "about rival card card2 price when",
	"gossipCaught": "about rival card card2 price when", "gossipKick": "about rival card card2 price when",
	"gossipTrade": "about rival card card2 price when", "gossipRipoff": "about rival card card2 price when",
	"gossipRivalry": "about rival card card2 price when", "gossipDuel": "about rival card card2 price when",
	"rivalSpot": "rival", "rivalAccept": "rival", "rivalWin": "rival", "rivalLose": "rival", "rivalHit": "rival card price",
	"rivalBrick": "rival", "rivalShamed": "rival",
	"chatOpenNews": "name", "chatMoan": "name", "chatMoanReply": "name", "chatBrag": "name", "chatBragReply": "name", "chatAskDeck": "name", "chatDeckReply": "name", "chatAskPull": "name", "chatPullReply": "name", "chatReactNice": "about when",
	"nickCoined": "name nick", "nickCoinedBad": "name nick", "nickTaken": "name nick", "nickLost": "name nick",
	"gossipNick": "about rival card card2 price when nick", "gossipNickBad": "about rival card card2 price when nick", "gossipNickTaken": "about rival card card2 price when nick",
	"empUpsell": "name", "upsellYes": "name", "tillGreetReply": "name", "tillThanksReply": "name", "tillGossipReply": "name", "empGreet": "name", "empThanks": "name", "empTillChat": "name", "empBounce": "name", "empChatMoan": "name", "empChatBrag": "name", "empChatDeck": "name", "empChatPull": "name", "empChatShop": "name", "tillShopReply": "name", "empGossip": "name", "empCaughtThief": "name", "tillChatReply": "name",
	"boardRead": "name nick", "boardBrag": "nick", "boardHere": "name", "boardSpent": "name price", "boardVisits": "name", "boardPacks": "name",
	"boardWins": "name", "boardPulls": "name price", "boardBoss": "name", "boardRivals": "name rival", "boardRivalMine": "rival",
	"boardGossip": "name", "boardShame": "name",
	"tableBanter": "name", "tableReply": "name", "friendMade": "name", "friendHello": "name", "friendSpot": "name", "friendHit": "name card price", "friendBrick": "name",
	"friendShamed": "name", "gossipFriends": "about rival card card2 price when",
}

func TestLineTokens(t *testing.T) {
	d, err := LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	tok := regexp.MustCompile(`\{(\w+)\}`)
	check := func(where, situation string, lines []string) {
		ok := map[string]bool{"shop": true}
		for _, k := range regexp.MustCompile(`\S+`).FindAllString(lineTokens[situation], -1) {
			ok[k] = true
		}
		for _, l := range lines {
			if strings.Contains(l, "—") {
				t.Errorf("%s %s: em dash in %q", where, situation, l)
			}
			for _, m := range tok.FindAllStringSubmatch(l, -1) {
				if !ok[m[1]] {
					t.Errorf("%s %s: {%s} isn't filled in this situation: %q", where, situation, m[1], l)
				}
			}
		}
	}
	for s, ls := range d.Lines {
		check("built-in", s, ls)
	}
	for _, tr := range d.Traits {
		for s, ls := range tr.Lines {
			check("trait "+tr.ID, s, ls)
		}
	}
	for s, ls := range d.EmployeeLines {
		check("employee", s, ls)
	}
	for _, p := range d.EmployeePerks {
		for s, ls := range p.Lines {
			check("perk "+p.ID, s, ls)
		}
	}
}
