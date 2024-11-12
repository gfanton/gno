package ui

type (
	H1   string
	H2   string
	H3   string
	H4   string
	H5   string
	H6   string
	Text string

	Quote     string
	Paragraph string
	Break     struct{} // Thematic Breaks

	// Alias
	Title  = H1
	Title2 = H2
	Title3 = H3
	Title4 = H4
	Title5 = H5
	Title6 = H6
)

func (text H1) String() string    { return "# " + string(text) }
func (text H2) String() string    { return "## " + string(text) }
func (text H3) String() string    { return "### " + string(text) }
func (text H4) String() string    { return "#### " + string(text) }
func (text H5) String() string    { return "##### " + string(text) }
func (text H6) String() string    { return "###### " + string(text) }
func (text Quote) String() string { return "> " + string(text) }
func (text Text) String() string  { return string(text) }
func (_ Break) String() string    { return "---" }

type Code struct {
	Lang   string
	Source string
}

func (c Code) String() string {
	return "```" + string(c.Lang) + "\n" +
		string(c.Source) +
		"```"
}
