package ui

import ui "gno.land/p/demo/ui/v2"

func Render(path string) string {
	bulletlist := ui.BulletList(
		ui.Join(ui.Text("a text: "), ui.Bold("Hello"), ui.Text(" noooo")),
		ui.Text("another text"),
		ui.BulletList(
			ui.Text("a sub text"),
			ui.BulletList(
				ui.Text("a sub sub text"),
			),
		),
	)

	return ui.Join(
		ui.Title("Hello"),
		ui.Paragraph("Simple UI demonstration."),
		bulletlist,
	).Render()
}
