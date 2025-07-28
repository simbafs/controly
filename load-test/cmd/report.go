package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// --- Lipgloss Styling ---
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(1).PaddingRight(1)

	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#43E6D6"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5E5E"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#32CD32"))

	// Table styles
	tableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	tableCell = lipgloss.NewStyle().
			Padding(0, 1)

	tableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
)

func renderTable(title string, data [][]string) string {
	headers := []string{"Metric", "Value"}
	columnWidths := []int{30, 30}

	// Header
	headerCells := make([]string, len(headers))
	for i, h := range headers {
		style := tableHeader.Width(columnWidths[i]).MaxWidth(columnWidths[i])
		headerCells[i] = style.Render(h)
	}
	headerRow := lipgloss.JoinHorizontal(lipgloss.Left, headerCells...)

	// Body
	bodyRows := make([]string, len(data))
	for i, row := range data {
		rowCells := make([]string, len(row))
		for j, cell := range row {
			style := tableCell.Width(columnWidths[j]).MaxWidth(columnWidths[j])
			rowCells[j] = style.Render(cell)
		}
		bodyRows[i] = lipgloss.JoinHorizontal(lipgloss.Left, rowCells...)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, bodyRows...)

	table := tableStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		headerRow,
		body,
	))

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		table,
	)
}

func GenerateAndPrintReport(n int, errorThreshold float64, result TestResult) {
	// --- Render Report ---
	fmt.Println("") // Clear the progress dots

	// --- Connection Report ---
	connectionData := [][]string{
		{"Requested Pairs", valueStyle.Render(fmt.Sprintf("%d", n))},
		{"Successful Controllers", successStyle.Render(fmt.Sprintf("%d", result.SuccessfulControllers))},
		{"Successful Displays", successStyle.Render(fmt.Sprintf("%d", result.SuccessfulDisplays))},
	}
	connectionTableStr := renderTable("Connection Report", connectionData)
	var connectionSummary string
	if uint64(n) != result.SuccessfulControllers || uint64(n) != result.SuccessfulDisplays {
		connectionSummary = errorStyle.Render(fmt.Sprintf("-> Connection mismatch! Requested: %d, Controllers: %d, Displays: %d",
			n, result.SuccessfulControllers, result.SuccessfulDisplays))
	} else {
		connectionSummary = successStyle.Render("-> All requested clients connected successfully.")
	}
	connectionReport := lipgloss.JoinVertical(lipgloss.Left, connectionTableStr, connectionSummary)

	// --- Throughput Report ---
	throughputData := [][]string{
		{"Controller Sent", valueStyle.Render(fmt.Sprintf("%d", result.ControllerCommandsSent)) + " commands"},
		{"Controller Received", valueStyle.Render(fmt.Sprintf("%d", result.ControllerStatusReceived)) + " status updates"},
		{"Display Sent", valueStyle.Render(fmt.Sprintf("%d", result.DisplayStatusSent)) + " status updates"},
		{"Display Received", valueStyle.Render(fmt.Sprintf("%d", result.DisplayCommandsReceived)) + " commands"},
	}
	throughputReport := renderTable("Throughput Report", throughputData)

	// --- Verification Report ---
	totalCommandAttempts := result.ControllerCommandsSent + result.ControllerWriteErrors
	commandLoss := int64(totalCommandAttempts) - int64(result.DisplayCommandsReceived)
	commandResult := successStyle.Render("OK")
	if commandLoss != 0 {
		commandResult = errorStyle.Render("MISMATCH")
	}

	totalStatusAttempts := result.DisplayStatusSent + result.DisplayWriteErrors
	statusLoss := int64(totalStatusAttempts) - int64(result.ControllerStatusReceived)
	statusResult := successStyle.Render("OK")
	if statusLoss != 0 {
		statusResult = errorStyle.Render("MISMATCH")
	}

	var commandLossRate float64
	if totalCommandAttempts > 0 {
		commandLossRate = float64(commandLoss) / float64(totalCommandAttempts) * 100
	}

	var statusLossRate float64
	if totalStatusAttempts > 0 {
		statusLossRate = float64(statusLoss) / float64(totalStatusAttempts) * 100
	}

	verificationData := [][]string{
		{"Command Attempts", fmt.Sprintf("%d", totalCommandAttempts)},
		{"Command Received", fmt.Sprintf("%d", result.DisplayCommandsReceived)},
		{"Command Loss", fmt.Sprintf("%d", commandLoss)},
		{"Command Loss Rate", fmt.Sprintf("%.2f%%", commandLossRate)},
		{"Command Result", commandResult},
		{"", ""}, // separator
		{"Status Attempts", fmt.Sprintf("%d", totalStatusAttempts)},
		{"Status Received", fmt.Sprintf("%d", result.ControllerStatusReceived)},
		{"Status Loss", fmt.Sprintf("%d", statusLoss)},
		{"Status Loss Rate", fmt.Sprintf("%.2f%%", statusLossRate)},
		{"Status Result", statusResult},
	}
	verificationReport := renderTable("Verification Report", verificationData)

	// --- Operational Errors ---
	opErrorsData := [][]string{
		{"Connection Errors", errorStyle.Render(fmt.Sprintf("%d", result.ConnectionErrors))},
		{"Subscribe Write Errors", errorStyle.Render(fmt.Sprintf("%d", result.SubscribeWriteErrors))},
		{"Controller Write Errors", errorStyle.Render(fmt.Sprintf("%d", result.ControllerWriteErrors))},
		{"Controller Read Errors", errorStyle.Render(fmt.Sprintf("%d", result.ControllerReadErrors))},
		{"Display Write Errors", errorStyle.Render(fmt.Sprintf("%d", result.DisplayWriteErrors))},
		{"Display Read Errors", errorStyle.Render(fmt.Sprintf("%d", result.DisplayReadErrors))},
	}
	opErrorsReport := renderTable("Operational Errors", opErrorsData)

	// --- Overall Error Rate ---
	totalErrors := result.ConnectionErrors + result.SubscribeWriteErrors + result.ControllerWriteErrors + result.ControllerReadErrors + result.DisplayWriteErrors + result.DisplayReadErrors
	if commandLoss > 0 {
		totalErrors += uint64(commandLoss)
	}
	if statusLoss > 0 {
		totalErrors += uint64(statusLoss)
	}

	totalConnectionAttempts := uint64(n * 2)
	totalSubscribeAttempts := result.SuccessfulControllers
	totalOpportunities := totalConnectionAttempts + totalSubscribeAttempts + totalCommandAttempts + totalStatusAttempts

	var overallErrorRate float64
	if totalOpportunities > 0 {
		overallErrorRate = (float64(totalErrors) / float64(totalOpportunities)) * 100
	}

	errorRateData := [][]string{
		{"Total Operations", valueStyle.Render(fmt.Sprintf("%d", totalOpportunities))},
		{"Total Errors", errorStyle.Render(fmt.Sprintf("%d", totalErrors))},
		{"Error Rate", valueStyle.Render(fmt.Sprintf("%.2f%%", overallErrorRate))},
		{"Error Threshold", valueStyle.Render(fmt.Sprintf("%.2f%%", errorThreshold))},
	}
	errorRateReport := renderTable("Overall Error Rate", errorRateData)

	// --- Final Verdict ---
	var finalVerdict string
	if overallErrorRate > errorThreshold {
		finalVerdict = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5E5E")).Render(
			fmt.Sprintf("Load test FAILED: Error rate %.2f%% exceeds threshold of %.2f%%.", overallErrorRate, errorThreshold),
		)
	} else {
		finalVerdict = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#32CD32")).Render(
			fmt.Sprintf("Load test PASSED: Error rate %.2f%% is within the threshold of %.2f%%.", overallErrorRate, errorThreshold),
		)
	}

	// --- Combine and Print ---
	mainContent := lipgloss.JoinVertical(lipgloss.Top,
		connectionReport,
		throughputReport,
		verificationReport,
		opErrorsReport,
		errorRateReport,
	)

	fmt.Println(lipgloss.NewStyle().Margin(1).Render(mainContent))
	fmt.Println(finalVerdict)
}
