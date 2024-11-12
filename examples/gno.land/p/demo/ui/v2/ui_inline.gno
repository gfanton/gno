package ui

func Bold(s string) string                { return "**" + string(s) + "**" }
func Italic(s string) string              { return "_" + string(s) + "_" }
func Strike(s string) string              { return "~~" + string(s) + "~~" }
func Link(text string, url string) string { return "[" + text + "](" + url + ")" }
func InlineCode(s string) string          { return "`" + string(s) + "`" }

type Image struct {
	AltText string
	URL     string
}

func Image(alttext, url string) string {
	return Image{AltText: alttext, URL: url}.String()
}

func (img Image) String() string {
	return "![" + img.AltText + "](" + img.URL + ")"
}
