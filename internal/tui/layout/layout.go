package layout

type Layout struct {
	container *Container
}

func New() *Layout {
	return &Layout{
		container: NewContainer(),
	}
}

func (l *Layout) SetSize(width, height int) {
	l.container.SetSize(width, height)
}

func (l *Layout) Width() int {
	return l.container.Width()
}

func (l *Layout) Height() int {
	return l.container.Height()
}

func (l *Layout) ContentHeight() int {
	return l.container.ContentHeight()
}

func (l *Layout) ContentWidth() int {
	return l.container.ContentWidth()
}

func (l *Layout) SetPRDTitle(title string) {
	l.container.TitleBar().SetTitle(title)
}

func (l *Layout) PRDTitle() string {
	return l.container.TitleBar().Title()
}

func (l *Layout) Render(headerData HeaderData, footerData FooterData, content string) string {
	return l.container.Render(headerData, footerData, content)
}
