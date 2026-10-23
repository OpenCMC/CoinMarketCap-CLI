package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type landingState int

const (
	landingHome landingState = iota
	landingMarkets
	landingTrending
)

type landingHeroCard struct {
	Available bool
	Name      string
	Symbol    string
	Price     float64
	Change24h float64
	Change7d  float64
	Has7d     bool
	Volume24h float64
	Chart     api.OHLCData
	ChartNote string
	Error     string
}

type landingHeroMsg struct {
	card landingHeroCard
	err  error
}

type LandingModel struct {
	client    *api.Client
	vs        string
	width     int
	height    int
	selection int
	state     landingState
	hero      landingHeroCard
	heroErr   error
	status    string
	markets   MarketsModel
	trending  TrendingModel
}

func NewLandingModel(client *api.Client) LandingModel {
	return LandingModel{
		client:    client,
		vs:        "USD",
		width:     120,
		height:    40,
		selection: 0,
		state:     landingHome,
		status:    "Loading CMC20…",
	}
}

func (m LandingModel) Init() tea.Cmd {
	return tea.Batch(m.fetchHero(), refreshTick())
}

func (m LandingModel) fetchHero() tea.Cmd {
	return func() tea.Msg {
		card, err := m.loadHero(context.Background())
		return landingHeroMsg{card: card, err: err}
	}
}

func (m LandingModel) loadHero(ctx context.Context) (landingHeroCard, error) {
	asset, err := m.client.ResolveBySlug(ctx, "coinmarketcap-20-index")
	if err != nil {
		return landingHeroCard{Available: false, Error: err.Error()}, err
	}

	id := strconv.FormatInt(asset.ID, 10)
	quotes, err := m.client.QuotesLatestByID(ctx, []string{id}, m.vs)
	if err != nil {
		return landingHeroCard{Available: false, Error: err.Error()}, err
	}
	quoteCoin, ok := quotes[id]
	if !ok {
		return landingHeroCard{Available: false, Error: "CMC20 quote not found"}, api.ErrAssetNotFound
	}
	quote, ok := quoteCoin.Quote[m.vs]
	if !ok {
		return landingHeroCard{Available: false, Error: "CMC20 quote unavailable for selected currency"}, api.ErrAssetNotFound
	}

	card := landingHeroCard{
		Available: true,
		Name:      defaultString(quoteCoin.Name, asset.Name),
		Symbol:    defaultString(quoteCoin.Symbol, asset.Symbol),
		Price:     quote.Price,
		Change24h: quote.PercentChange24h,
		Volume24h: quote.Volume24h,
	}

	now := time.Now().UTC()
	historical, histErr := m.client.QuotesHistoricalByID(ctx, id, m.vs, now.Add(-7*24*time.Hour), now, 8, "daily")
	if histErr == nil && historical != nil && len(historical.Quotes) > 0 {
		card.Chart = quoteHistoryToOHLC(historical, m.vs)
		if len(card.Chart) > 0 {
			if baseline := card.Chart[0][4]; baseline > 0 {
				card.Change7d = ((quote.Price - baseline) / baseline) * 100
				card.Has7d = true
			}
		}
	}
	if len(card.Chart) == 0 {
		card.ChartNote = "history unavailable"
		if errors.Is(histErr, api.ErrKeyRequired) {
			card.ChartNote = "7D history needs a free API key (cmc auth)"
		}
	}

	return card, nil
}

func (m LandingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == landingMarkets {
			updated, cmd := m.markets.Update(msg)
			m.markets = updated.(MarketsModel)
			return m, cmd
		}
		if m.state == landingTrending {
			updated, cmd := m.trending.Update(msg)
			m.trending = updated.(TrendingModel)
			return m, cmd
		}
		return m, nil

	case landingHeroMsg:
		m.heroErr = msg.err
		m.hero = msg.card
		if msg.err != nil {
			m.status = "CMC20 unavailable"
			if msg.card.Error != "" {
				m.status = fmt.Sprintf("CMC20 unavailable: %s", msg.card.Error)
			}
		} else if len(msg.card.Chart) > 0 {
			m.status = "CMC20 updated"
		} else {
			m.status = "CMC20 updated without history"
		}
		return m, nil

	case refreshTickMsg:
		if m.state == landingMarkets {
			updated, cmd := m.markets.Update(msg)
			m.markets = updated.(MarketsModel)
			return m, cmd
		}
		if m.state == landingTrending {
			updated, cmd := m.trending.Update(msg)
			m.trending = updated.(TrendingModel)
			return m, cmd
		}
		m.status = "Refreshing CMC20…"
		return m, tea.Batch(m.fetchHero(), refreshTick())

	case coinInfoMsg, quoteDetailMsg, ohlcMsg:
		if m.state == landingMarkets {
			updated, cmd := m.markets.Update(msg)
			m.markets = updated.(MarketsModel)
			if m.markets.ReturnToLanding {
				m.state = landingHome
				m.markets = MarketsModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}
		if m.state == landingTrending {
			updated, cmd := m.trending.Update(msg)
			m.trending = updated.(TrendingModel)
			if m.trending.ReturnToLanding {
				m.state = landingHome
				m.trending = TrendingModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}

	case coinsLoadedMsg:
		if m.state == landingMarkets {
			updated, cmd := m.markets.Update(msg)
			m.markets = updated.(MarketsModel)
			if m.markets.ReturnToLanding {
				m.state = landingHome
				m.markets = MarketsModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}

	case trendingLoadedMsg:
		if m.state == landingTrending {
			updated, cmd := m.trending.Update(msg)
			m.trending = updated.(TrendingModel)
			if m.trending.ReturnToLanding {
				m.state = landingHome
				m.trending = TrendingModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}

	case tea.KeyMsg:
		if m.state == landingMarkets {
			updated, cmd := m.markets.Update(msg)
			m.markets = updated.(MarketsModel)
			if m.markets.ReturnToLanding {
				m.state = landingHome
				m.markets = MarketsModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}
		if m.state == landingTrending {
			updated, cmd := m.trending.Update(msg)
			m.trending = updated.(TrendingModel)
			if m.trending.ReturnToLanding {
				m.state = landingHome
				m.trending = TrendingModel{}
				m.status = "Returned to landing"
				return m, refreshTick()
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.selection < 1 {
				m.selection++
			}
		case "k", "up":
			if m.selection > 0 {
				m.selection--
			}
		case "enter":
			if m.selection == 0 {
				m.state = landingMarkets
				m.markets = NewMarketsModel(m.client, "USD", 50, closeToLanding)
				m.markets.width, m.markets.height = normalizeViewSize(m.width, m.height)
				return m, m.markets.Init()
			}
			m.state = landingTrending
			m.trending = NewTrendingModel(m.client, "USD", closeToLanding)
			m.trending.width, m.trending.height = normalizeViewSize(m.width, m.height)
			return m, m.trending.Init()
		}
	}

	return m, nil
}

func (m LandingModel) View() string {
	width, height := normalizeViewSize(m.width, m.height)

	if m.state == landingMarkets {
		return m.markets.View()
	}
	if m.state == landingTrending {
		return m.trending.View()
	}

	brand := landingLogoView(width - 4)
	subhead := LandingSubheadStyle.Width(width - 4).Render("Professional data for AI agents and terminal workflows")

	hero, chart := m.homeCards(width, height)
	menu := m.menuView(width - 8)
	row := hero
	if chart != "" {
		if width < 110 {
			row = lipgloss.JoinVertical(lipgloss.Left, hero, "", chart)
		} else {
			row = lipgloss.JoinHorizontal(lipgloss.Top, hero, strings.Repeat(" ", 2), chart)
		}
	}

	body := []string{
		"",
		brand,
		subhead,
		"",
		row,
		"",
		menu,
		"",
		DimStyle.Render("  Enter open    Esc quit    j/k or arrows move"),
	}

	return renderFrame(width, height, strings.Join(body, "\n"))
}

func (m LandingModel) heroCardView(width int) string {
	if width < 40 {
		width = 40
	}

	lines := []string{
		LandingCardStyle.Width(width).Render(buildHeroCardBody(m.hero, m.heroErr, m.vs)),
	}
	return strings.Join(lines, "\n")
}

func (m LandingModel) chartCardView(width, height int) string {
	if width < 30 {
		width = 30
	}
	if height < 12 {
		height = 12
	}

	var b strings.Builder
	b.WriteString(LandingEntryStyle.Render(" CMC20 7D "))
	b.WriteString("\n")

	switch {
	case !m.hero.Available && m.hero.Error == "" && m.heroErr == nil:
		b.WriteString("\n")
		b.WriteString(DimStyle.Render("  Loading CMC20 7D chart…"))
	case len(m.hero.Chart) == 0:
		b.WriteString("\n")
		b.WriteString(DimStyle.Render("  No chart data available"))
		if m.hero.ChartNote != "" {
			b.WriteString("\n")
			b.WriteString(DimStyle.Render("  " + m.hero.ChartNote))
		}
	default:
		chartWidth := width - 8
		chartHeight := height / 3
		if chartHeight > 16 {
			chartHeight = 16
		}
		if chartHeight < 10 {
			chartHeight = 10
		}
		if chartWidth < 20 {
			chartWidth = 20
		}
		b.WriteString("\n")
		b.WriteString(renderBrailleChart(m.hero.Chart, chartWidth, chartHeight, m.vs, chartWindow7D))
		if m.hero.ChartNote != "" {
			b.WriteString("\n")
			b.WriteString(DimStyle.Render("  " + m.hero.ChartNote))
		}
	}

	return LandingCardStyle.Width(width).Render(b.String())
}

func (m LandingModel) homeCards(width, height int) (string, string) {
	width, height = normalizeViewSize(width, height)
	contentWidth := width - 8
	if contentWidth < 84 {
		contentWidth = 84
	}
	if width < 110 {
		return m.heroCardView(contentWidth), m.chartCardView(contentWidth, height-14)
	}

	leftWidth := contentWidth/3 + 10
	if leftWidth < 42 {
		leftWidth = 42
	}
	rightWidth := contentWidth - leftWidth - 2
	if rightWidth < 30 {
		rightWidth = 30
		leftWidth = contentWidth - rightWidth - 2
	}

	hero := m.heroCardView(leftWidth)
	chart := m.chartCardView(rightWidth, height-12)
	return hero, chart
}

func buildHeroCardBody(hero landingHeroCard, heroErr error, vs string) string {
	var b strings.Builder
	b.WriteString(LandingEntryStyle.Render(" CMC20 "))
	b.WriteString("\n")

	if !hero.Available {
		b.WriteString("\n")
		if hero.Error == "" && heroErr == nil {
			b.WriteString(DimStyle.Render("  Loading CMC20…"))
			return b.String()
		}
		b.WriteString(DimStyle.Render("  CMC20 unavailable"))
		if hero.Error != "" {
			b.WriteString("\n")
			b.WriteString(DimStyle.Render("  " + hero.Error))
		} else if heroErr != nil {
			b.WriteString("\n")
			b.WriteString(DimStyle.Render("  " + heroErr.Error()))
		}
		return b.String()
	}

	name := hero.Name
	if name == "" {
		name = "CMC20"
	}
	fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("Name"), display.SanitizeCell(name))
	fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("Symbol"), display.FormatSymbol(hero.Symbol))
	fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("Price"), display.FormatPrice(hero.Price, vs))
	fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("24h"), ColorPercent(hero.Change24h, display.FormatPercent(hero.Change24h)))
	if hero.Has7d {
		fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("7d"), ColorPercent(hero.Change7d, display.FormatPercent(hero.Change7d)))
	} else {
		fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("7d"), DimStyle.Render("—"))
	}
	fmt.Fprintf(&b, "  %s %s\n", LabelStyle.Render("Volume 24h"), display.FormatLargeNumber(hero.Volume24h, vs))
	if hero.Error != "" {
		b.WriteString("\n")
		b.WriteString(DimStyle.Render("  " + hero.Error))
	}
	return b.String()
}

func (m LandingModel) menuView(width int) string {
	if width < 40 {
		width = 40
	}
	var b strings.Builder
	b.WriteString(LandingEntryStyle.Render(" Main Paths "))
	b.WriteString("\n\n")

	top := "Top 50"
	trending := "Trending 50"
	if m.selection == 0 {
		top = HighlightSymbol + top
		top = SelectedStyle.Render(top)
		trending = "  " + trending
	} else {
		top = "  " + top
		trending = HighlightSymbol + trending
		trending = SelectedStyle.Render(trending)
	}
	b.WriteString(top)
	b.WriteString("\n")
	b.WriteString(trending)
	b.WriteString("\n\n")
	b.WriteString(DimStyle.Render("  markets and trending reuse the existing list views"))
	return LandingCardStyle.Width(width).Render(b.String())
}

func defaultString(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
