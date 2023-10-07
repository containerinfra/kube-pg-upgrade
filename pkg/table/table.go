package table

import (
	"os"

	"github.com/olekukonko/tablewriter"
)

func Print(header []string, body [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	if len(header) > 0 {
		table.SetHeader(header)
	}
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	if len(body) > 0 {
		table.AppendBulk(body)
	}

	table.Render()
}
