package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultUser = "viimaru"
	outputFile  = "assets/metrics.svg"
)

type user struct {
	CreatedAt time.Time `json:"created_at"`
}

type repository struct {
	Name      string    `json:"name"`
	Fork      bool      `json:"fork"`
	Archived  bool      `json:"archived"`
	PushedAt  time.Time `json:"pushed_at"`
	Languages string    `json:"languages_url"`
}

type language struct {
	Name  string
	Bytes int64
}

type metrics struct {
	User       string
	Public     int
	Active     int
	Since      int
	LastPush   string
	Languages  []language
	TotalBytes int64
	Generated  string
}

func main() {
	name := os.Getenv("METRICS_USER")
	if name == "" {
		name = defaultUser
	}

	client := &http.Client{Timeout: 20 * time.Second}
	account := user{}
	get(client, "https://api.github.com/users/"+name, &account)

	var repos []repository
	for page := 1; ; page++ {
		var batch []repository
		get(client, fmt.Sprintf("https://api.github.com/users/%s/repos?type=owner&sort=updated&per_page=100&page=%d", name, page), &batch)
		repos = append(repos, batch...)
		if len(batch) < 100 {
			break
		}
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(-1, 0, 0)
	languageBytes := map[string]int64{}
	m := metrics{User: name, Public: len(repos), Since: account.CreatedAt.Year(), Generated: now.Format("2006-01-02")}

	for _, repo := range repos {
		if repo.PushedAt.After(parseDate(m.LastPush)) {
			m.LastPush = repo.PushedAt.Format("2006-01-02")
		}
		if !repo.Fork && !repo.Archived && repo.PushedAt.After(cutoff) {
			m.Active++
		}
		if repo.Fork || repo.Archived {
			continue
		}
		var langs map[string]int64
		get(client, repo.Languages, &langs)
		for lang, bytes := range langs {
			languageBytes[lang] += bytes
			m.TotalBytes += bytes
		}
	}

	for name, bytes := range languageBytes {
		m.Languages = append(m.Languages, language{Name: name, Bytes: bytes})
	}
	sort.Slice(m.Languages, func(i, j int) bool { return m.Languages[i].Bytes > m.Languages[j].Bytes })
	if len(m.Languages) > 5 {
		var other int64
		for _, lang := range m.Languages[4:] {
			other += lang.Bytes
		}
		m.Languages = append(m.Languages[:4], language{Name: "other", Bytes: other})
	}

	if err := os.WriteFile(outputFile, []byte(render(m)), 0o644); err != nil {
		fatal(err)
	}
}

func get(client *http.Client, url string, target any) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fatal(err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "viimaru-profile-metrics")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		fatal(fmt.Errorf("GitHub API %s: %s", res.Status, strings.TrimSpace(string(body))))
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		fatal(err)
	}
}

func parseDate(value string) time.Time {
	date, _ := time.Parse("2006-01-02", value)
	return date
}

func render(m metrics) string {
	var out strings.Builder
	out.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="760" height="250" viewBox="0 0 760 250" role="img" aria-labelledby="title desc">
  <title id="title">Public GitHub activity for ` + xmlText(m.User) + `</title>
  <desc id="desc">Public repository activity and language volume, generated from the GitHub API.</desc>
  <style>
    .bg{fill:#282828}.panel{fill:#1d2021}.edge{stroke:#504945}.soft{fill:#3c3836}.fg{fill:#ebdbb2}.muted{fill:#a89984}.faint{fill:#665c54}.green{fill:#b8bb26}.aqua{fill:#83a598}.yellow{fill:#fabd2f}.orange{fill:#fe8019}.red{fill:#fb4934}
    text{font-family:ui-monospace,SFMono-Regular,Consolas,"Liberation Mono",monospace}
    @media (prefers-color-scheme:light){.bg{fill:#fbf1c7}.panel{fill:#f2e5bc}.edge{stroke:#bdae93}.soft{fill:#ebdbb2}.fg{fill:#3c3836}.muted{fill:#665c54}.faint{fill:#a89984}.green{fill:#79740e}.aqua{fill:#076678}.yellow{fill:#b57614}.orange{fill:#af3a03}.red{fill:#9d0006}}
  </style>
  <rect x="8" y="8" width="744" height="234" rx="10" class="bg edge" stroke-width="3"/>
  <rect x="22" y="22" width="716" height="32" rx="5" class="panel"/>
  <circle cx="38" cy="38" r="4" class="red"/><circle cx="52" cy="38" r="4" class="yellow"/><circle cx="66" cy="38" r="4" class="green"/>
  <text x="84" y="43" class="fg" font-size="13">git pulse --public @` + xmlText(m.User) + `</text>
  <text x="719" y="43" text-anchor="end" class="faint" font-size="10">` + m.Generated + `</text>

  <!-- deliberately crooked little commit graph -->
  <g fill="none" stroke-width="3" stroke-linecap="round">
    <path d="M55 82v105c0 18 20 16 20 30" class="edge"/>
    <path d="M55 105c0 17 35 9 35 30v31" stroke="#d79921"/>
    <path d="M55 151c0 14-20 10-20 28v17" stroke="#458588"/>
  </g>
  <g class="bg" stroke-width="3">
    <circle cx="55" cy="82" r="7" stroke="#98971a"/><circle cx="55" cy="105" r="7" stroke="#98971a"/><circle cx="90" cy="135" r="7" stroke="#d79921"/><circle cx="90" cy="166" r="7" stroke="#d79921"/><circle cx="55" cy="151" r="7" stroke="#98971a"/><circle cx="35" cy="196" r="7" stroke="#458588"/><circle cx="75" cy="217" r="7" stroke="#98971a"/>
  </g>
  <text x="113" y="91" class="muted" font-size="11">refs/public</text>
  <text x="113" y="112" class="fg" font-size="18">` + strconv.Itoa(m.Public) + `</text>
  <text x="113" y="145" class="muted" font-size="11">active@12mo</text>
  <text x="113" y="166" class="fg" font-size="18">` + strconv.Itoa(m.Active) + `</text>
  <text x="113" y="199" class="muted" font-size="11">first seen</text>
  <text x="113" y="220" class="fg" font-size="18">` + strconv.Itoa(m.Since) + `</text>

  <path d="M218 72v151" class="edge" stroke-width="2"/>
  <text x="242" y="88" class="aqua" font-size="12">language volume</text>
  <text x="718" y="88" text-anchor="end" class="faint" font-size="10">bytes · source, non-archived repos</text>
`)

	colors := []string{"green", "aqua", "yellow", "orange", "red"}
	if len(m.Languages) == 0 {
		out.WriteString(`  <rect x="242" y="108" width="476" height="68" rx="5" class="panel edge" stroke-width="2" stroke-dasharray="5 5"/>
  <text x="262" y="136" class="yellow" font-size="13">∅  no language bytes indexed</text>
  <text x="262" y="158" class="muted" font-size="11">empty tree; still a valid tree.</text>
`)
	}
	for i, lang := range m.Languages {
		y := 114 + i*24
		pct := 0.0
		if m.TotalBytes > 0 {
			pct = float64(lang.Bytes) / float64(m.TotalBytes) * 100
		}
		width := pct * 3.1
		out.WriteString(fmt.Sprintf("  <text x=\"242\" y=\"%d\" class=\"fg\" font-size=\"11\">%-12s</text>\n", y+4, xmlText(trim(lang.Name, 12))))
		out.WriteString(fmt.Sprintf("  <rect x=\"340\" y=\"%d\" width=\"310\" height=\"9\" rx=\"2\" class=\"soft\"/>\n", y-5))
		out.WriteString(fmt.Sprintf("  <rect x=\"340\" y=\"%d\" width=\"%.1f\" height=\"9\" rx=\"2\" class=\"%s\"/>\n", y-5, width, colors[i]))
		out.WriteString(fmt.Sprintf("  <text x=\"718\" y=\"%d\" text-anchor=\"end\" class=\"muted\" font-size=\"11\">%.1f%%</text>\n", y+4, pct))
	}

	out.WriteString(`  <text x="242" y="226" class="faint" font-size="10">last push ` + xmlText(m.LastPush) + ` · bars measure repository bytes, not skill</text>
</svg>
`)
	return out.String()
}

func trim(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

func xmlText(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "update-metrics:", err)
	os.Exit(1)
}
