package main

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"io"
	"net/http"
	"os"
	"strings"
)

var Reset = "\033[0m"
var Highlight = "\033[7m"
var Red = "\033[31m"
var Green = "\033[32m"
var Newline = "\n"

var pageTitle = ""
var url = ""

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

type model struct {
	content          string
	selected         int
	pageTitle        string
	ready            bool
	mainContentWidth int
	viewport         viewport.Model
}

type link struct {
	position int
	text     string
	url      string
}

var pageLinks []link

func (m model) Init() tea.Cmd {
	return nil
}

func highlightLink(content string, index int) string {
	var buffer bytes.Buffer
	var linkCount = 0

	var greenCompare = []rune(Green)

	for _, rune := range content {
		var char = string(rune)
		if rune == greenCompare[0] {
			if linkCount == index {
				buffer.WriteString(Highlight)
			}
			linkCount += 1
		}
		buffer.WriteString(char)
	}

	return buffer.String()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

		if k := msg.String(); k == "n" {
			m.selected += 2
			s := highlightLink(wordwrap.String(m.content, m.mainContentWidth), m.selected)
			m.viewport.SetContent(s)
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.mainContentWidth = msg.Width - 2

			s := highlightLink(wordwrap.String(m.content, m.mainContentWidth), m.selected)
			m.viewport.SetContent(s)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m model) headerView() string {
	title := titleStyle.Render(pageTitle)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func loadBody() string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		fmt.Println("Error fetching URL:", err)
		return ""
	}

	req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")

	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Error fetching URL:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}

	return string(body)
}

func parseTextFromNode(n html.Node) string {
	if n.DataAtom == atom.A {
		return Green + n.FirstChild.Data + Reset
	}

	if n.Parent.DataAtom == atom.A {
		return ""
	}

	if n.Type == html.TextNode {
		return n.Data
	}

	return ""
}

func getText(node html.Node) string {
	var buffer bytes.Buffer

	buffer.WriteString(parseTextFromNode(node))

	for n := range node.Descendants() {
		buffer.WriteString(parseTextFromNode(*n))
	}

	return buffer.String()
}

func parseHtml(htmlString string) string {
	s := htmlString
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		fmt.Println("Error reading response body:", err)
	}
	var buffer bytes.Buffer

	for n := range doc.Descendants() {
		for _, attr := range n.Attr {
			if attr.Key == "class" && attr.Val == "mw-page-title-main" {
				pageTitle = getText(*n)
			}
		}

		if n.Type == html.ElementNode {
			if n.DataAtom == atom.H2 {
				buffer.WriteString(Red + getText(*n) + Reset + Newline)
			}
		}

		if n.DataAtom == atom.P {
			buffer.WriteString(getText(*n) + Newline)
		}
	}

	return buffer.String()
}

func main() {
	// Load some text for our viewport
	// content := parseHtml(loadBody())
	if len(os.Args) < 2 {
		fmt.Println("Please input a url")
		return
	}

	fmt.Println(os.Args[0])

	url = os.Args[1]

	content := parseHtml(loadBody())
	p := tea.NewProgram(
		model{content: string(content), pageTitle: string(pageTitle), selected: 0},
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
}
