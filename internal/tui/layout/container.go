package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Container struct {
	width    int
	height   int
	header   *Header
	titleBar *TitleBar
	footer   *Footer
}

func NewContainer() *Container {
	return &Container{
		header:   NewHeader(),
		titleBar: NewTitleBar(),
		footer:   NewFooter(),
	}
}

func (c *Container) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.header.SetWidth(width)
	c.titleBar.SetWidth(width)
	c.footer.SetWidth(width)
}

func (c *Container) Width() int {
	return c.width
}

func (c *Container) Height() int {
	return c.height
}

func (c *Container) ContentHeight() int {
	usedHeight := c.header.Height() + c.footer.Height()
	if c.titleBar.Title() != "" {
		usedHeight += c.titleBar.Height()
	}
	available := c.height - usedHeight
	if available < 1 {
		return 1
	}
	return available
}

func (c *Container) ContentWidth() int {
	if c.width <= 4 {
		return c.width
	}
	return c.width - 4
}

func (c *Container) Header() *Header {
	return c.header
}

func (c *Container) Footer() *Footer {
	return c.footer
}

func (c *Container) TitleBar() *TitleBar {
	return c.titleBar
}

func (c *Container) Render(headerData HeaderData, footerData FooterData, content string) string {
	headerStr := c.header.Render(headerData)
	footerStr := c.footer.Render(footerData)

	contentStyle := lipgloss.NewStyle().
		Width(c.width).
		Height(c.ContentHeight()).
		Padding(0, 1)

	contentLines := strings.Split(content, "\n")
	maxLines := c.ContentHeight()

	if len(contentLines) > maxLines {
		contentLines = contentLines[:maxLines]
	}

	paddedContent := strings.Join(contentLines, "\n")
	contentStr := contentStyle.Render(paddedContent)

	var result string
	result = headerStr + "\n"
	if c.titleBar.Title() != "" {
		result += c.titleBar.Render() + "\n"
	}
	result += contentStr + "\n" + footerStr

	return result
}

func (c *Container) RenderContent(content string) string {
	contentStyle := lipgloss.NewStyle().
		Width(c.width).
		Padding(0, 1)

	return contentStyle.Render(content)
}
