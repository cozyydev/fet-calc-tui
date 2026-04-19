package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	salesTaxRate       = 0.0875
	fetRate            = 0.11
	fflXferFee         = 150.00
	pdPickupPerFirearm = 90.00
	partsPackageFee    = 25.00
	barrelsPackageFee  = 50.00
)

type tab int

const (
	tabFirearms tab = iota
	tabParts
	tabBarrels
	tabCount
)

type field int

const (
	fieldPrice field = iota
	fieldShipping
	fieldSalesTaxPaid
	fieldFETPaid
	fieldAdditionalFees
	fieldTotalPaid
	fieldFFLXferQty
	fieldPDPickupQty
	fieldPackageQty
	fieldCount
)

type calcResult struct {
	SalesTaxBase       float64
	FETBase            float64
	ExpectedSalesTax   float64
	ExpectedFET        float64
	ExpectedTotal      float64
	InvoiceDiscrepancy float64
	SalesTaxDue        float64
	FETDue             float64
	TotalDue           float64
	ServiceFees        float64
	GrandTotalDue      float64
	Error              string
}

type model struct {
	activeTab       tab
	inputs          map[tab][]textinput.Model
	focusOrder      map[tab][]field
	focusIndex      map[tab]int
	results         map[tab]calcResult
	fflXferChecked  bool
	pdPickupChecked bool
}

func parseMoney(s string) (float64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

func parseWholeNumber(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func money(v float64) string {
	return fmt.Sprintf("$%.2f", round2(v))
}

func signedMoney(v float64) string {
	v = round2(v)
	if v >= 0 {
		return fmt.Sprintf("+$%.2f", v)
	}
	return fmt.Sprintf("-$%.2f", math.Abs(v))
}

func tabName(t tab) string {
	switch t {
	case tabFirearms:
		return "Firearms"
	case tabParts:
		return "Parts"
	case tabBarrels:
		return "Barrels"
	default:
		return ""
	}
}

func itemLabel(t tab) string {
	switch t {
	case tabFirearms:
		return "Firearm price"
	case tabParts:
		return "Parts price"
	case tabBarrels:
		return "Barrels price"
	default:
		return "Item price"
	}
}

func fetApplies(t tab) bool {
	return t == tabFirearms
}

func calculate(
	t tab,
	priceStr string,
	shippingStr string,
	paidSalesTaxStr string,
	paidFETStr string,
	additionalFeesStr string,
	totalPaidStr string,
	fflXferChecked bool,
	fflXferQtyStr string,
	pdPickupChecked bool,
	pdPickupQtyStr string,
	pkgQtyStr string,
) calcResult {
	var r calcResult

	price, err := parseMoney(priceStr)
	if err != nil {
		r.Error = "invalid item price"
		return r
	}

	shipping, err := parseMoney(shippingStr)
	if err != nil {
		r.Error = "invalid shipping/handling"
		return r
	}

	paidSalesTax, err := parseMoney(paidSalesTaxStr)
	if err != nil {
		r.Error = "invalid sales tax on invoice"
		return r
	}

	paidFET, err := parseMoney(paidFETStr)
	if err != nil {
		r.Error = "invalid FET on invoice"
		return r
	}

	additionalFees, err := parseMoney(additionalFeesStr)
	if err != nil {
		r.Error = "invalid additional fees on invoice"
		return r
	}

	totalPaid, err := parseMoney(totalPaidStr)
	if err != nil {
		r.Error = "invalid total paid"
		return r
	}

	fflQty, err := parseWholeNumber(fflXferQtyStr)
	if err != nil {
		r.Error = "invalid FFL Xfer firearm quantity"
		return r
	}

	pdQty, err := parseWholeNumber(pdPickupQtyStr)
	if err != nil {
		r.Error = "invalid PD Pickup firearm quantity"
		return r
	}

	pkgQty, err := parseWholeNumber(pkgQtyStr)
	if err != nil {
		r.Error = "invalid package / invoice quantity"
		return r
	}

	if price < 0 || shipping < 0 || paidSalesTax < 0 || paidFET < 0 ||
		additionalFees < 0 || totalPaid < 0 || fflQty < 0 || pdQty < 0 ||
		pkgQty < 0 {
		r.Error = "amounts cannot be negative"
		return r
	}

	if !fflXferChecked {
		fflQty = 0
	}
	if !pdPickupChecked {
		pdQty = 0
	}

	salesTaxBase := price + additionalFees
	fetBase := 0.0
	expectedFET := 0.0
	fetDue := 0.0

	if fetApplies(t) {
		fetBase = price + shipping + additionalFees
		expectedFET = fetBase * fetRate
		fetDue = expectedFET - paidFET
		if fetDue < 0 {
			fetDue = 0
		}
	} else {
		paidFET = 0
	}

	expectedSalesTax := salesTaxBase * salesTaxRate
	expectedTotal := salesTaxBase + expectedSalesTax + expectedFET
	invoiceDiscrepancy := totalPaid - expectedTotal

	salesTaxDue := expectedSalesTax - paidSalesTax
	if salesTaxDue < 0 {
		salesTaxDue = 0
	}

	serviceFees := 0.0
	switch t {
	case tabFirearms:
		serviceFees += float64(fflQty) * fflXferFee
		serviceFees += float64(pdQty) * pdPickupPerFirearm
	case tabParts:
		serviceFees += float64(pkgQty) * partsPackageFee
	case tabBarrels:
		serviceFees += float64(pkgQty) * barrelsPackageFee
	}

	totalDue := salesTaxDue + fetDue
	grandTotalDue := totalDue + serviceFees

	r.SalesTaxBase = round2(salesTaxBase)
	r.FETBase = round2(fetBase)
	r.ExpectedSalesTax = round2(expectedSalesTax)
	r.ExpectedFET = round2(expectedFET)
	r.ExpectedTotal = round2(expectedTotal)
	r.InvoiceDiscrepancy = round2(invoiceDiscrepancy)
	r.SalesTaxDue = round2(salesTaxDue)
	r.FETDue = round2(fetDue)
	r.TotalDue = round2(totalDue)
	r.ServiceFees = round2(serviceFees)
	r.GrandTotalDue = round2(grandTotalDue)

	return r
}

func newInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Width = 32
	in.CharLimit = 32
	return in
}

func newInputsForTab() []textinput.Model {
	inputs := make([]textinput.Model, fieldCount)
	inputs[fieldPrice] = newInput("Item price, e.g. 650.00")
	inputs[fieldShipping] = newInput("Shipping/handling, blank = 0")
	inputs[fieldSalesTaxPaid] = newInput("Sales tax on invoice, blank = 0")
	inputs[fieldFETPaid] = newInput("FET on invoice, blank = 0")
	inputs[fieldAdditionalFees] = newInput("Additional fees on invoice, blank = 0")
	inputs[fieldTotalPaid] = newInput("Total paid, blank = 0")
	inputs[fieldFFLXferQty] = newInput("How many firearms?")
	inputs[fieldPDPickupQty] = newInput("How many firearms?")
	inputs[fieldPackageQty] = newInput("How many packages / invoices received?")
	return inputs
}

func initialModel() model {
	inputs := map[tab][]textinput.Model{
		tabFirearms: newInputsForTab(),
		tabParts:    newInputsForTab(),
		tabBarrels:  newInputsForTab(),
	}

	focusOrder := map[tab][]field{
		tabFirearms: {
			fieldPrice,
			fieldShipping,
			fieldSalesTaxPaid,
			fieldFETPaid,
			fieldAdditionalFees,
			fieldTotalPaid,
			fieldFFLXferQty,
			fieldPDPickupQty,
		},
		tabParts: {
			fieldPrice,
			fieldShipping,
			fieldSalesTaxPaid,
			fieldAdditionalFees,
			fieldTotalPaid,
			fieldPackageQty,
		},
		tabBarrels: {
			fieldPrice,
			fieldShipping,
			fieldSalesTaxPaid,
			fieldAdditionalFees,
			fieldTotalPaid,
			fieldPackageQty,
		},
	}

	inputs[tabFirearms][fieldPrice].Focus()

	m := model{
		activeTab:  tabFirearms,
		inputs:     inputs,
		focusOrder: focusOrder,
		focusIndex: map[tab]int{
			tabFirearms: 0,
			tabParts:    0,
			tabBarrels:  0,
		},
		results: map[tab]calcResult{
			tabFirearms: {},
			tabParts:    {},
			tabBarrels:  {},
		},
		fflXferChecked:  false,
		pdPickupChecked: false,
	}

	for _, t := range []tab{tabFirearms, tabParts, tabBarrels} {
		m.results[t] = calculate(
			t,
			m.inputs[t][fieldPrice].Value(),
			m.inputs[t][fieldShipping].Value(),
			m.inputs[t][fieldSalesTaxPaid].Value(),
			m.inputs[t][fieldFETPaid].Value(),
			m.inputs[t][fieldAdditionalFees].Value(),
			m.inputs[t][fieldTotalPaid].Value(),
			m.fflXferChecked,
			m.inputs[t][fieldFFLXferQty].Value(),
			m.pdPickupChecked,
			m.inputs[t][fieldPDPickupQty].Value(),
			m.inputs[t][fieldPackageQty].Value(),
		)
	}

	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) setFocusForActiveTab() {
	fields := m.focusOrder[m.activeTab]
	activeField := fields[m.focusIndex[m.activeTab]]

	for i := range m.inputs[m.activeTab] {
		if field(i) == activeField {
			m.inputs[m.activeTab][i].Focus()
		} else {
			m.inputs[m.activeTab][i].Blur()
		}
	}
}

func (m *model) recalcActive() {
	t := m.activeTab
	m.results[t] = calculate(
		t,
		m.inputs[t][fieldPrice].Value(),
		m.inputs[t][fieldShipping].Value(),
		m.inputs[t][fieldSalesTaxPaid].Value(),
		m.inputs[t][fieldFETPaid].Value(),
		m.inputs[t][fieldAdditionalFees].Value(),
		m.inputs[t][fieldTotalPaid].Value(),
		m.fflXferChecked,
		m.inputs[t][fieldFFLXferQty].Value(),
		m.pdPickupChecked,
		m.inputs[t][fieldPDPickupQty].Value(),
		m.inputs[t][fieldPackageQty].Value(),
	)
}

func (m *model) clearActive() {
	for i := range m.inputs[m.activeTab] {
		m.inputs[m.activeTab][i].SetValue("")
	}
	if m.activeTab == tabFirearms {
		m.fflXferChecked = false
		m.pdPickupChecked = false
	}
	m.recalcActive()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "left", "h":
			m.activeTab--
			if m.activeTab < 0 {
				m.activeTab = tabCount - 1
			}
			m.setFocusForActiveTab()
			m.recalcActive()
			return m, nil

		case "right", "l":
			m.activeTab++
			if m.activeTab >= tabCount {
				m.activeTab = 0
			}
			m.setFocusForActiveTab()
			m.recalcActive()
			return m, nil

		case "tab", "down":
			m.focusIndex[m.activeTab]++
			if m.focusIndex[m.activeTab] >= len(m.focusOrder[m.activeTab]) {
				m.focusIndex[m.activeTab] = 0
			}
			m.setFocusForActiveTab()
			return m, nil

		case "shift+tab", "up":
			m.focusIndex[m.activeTab]--
			if m.focusIndex[m.activeTab] < 0 {
				m.focusIndex[m.activeTab] = len(m.focusOrder[m.activeTab]) - 1
			}
			m.setFocusForActiveTab()
			return m, nil

		case "enter":
			m.recalcActive()
			return m, nil

		case "esc":
			m.clearActive()
			return m, nil

		case "f":
			if m.activeTab == tabFirearms {
				m.fflXferChecked = !m.fflXferChecked
				if !m.fflXferChecked {
					m.inputs[m.activeTab][fieldFFLXferQty].SetValue("")
				}
				m.recalcActive()
			}
			return m, nil

		case "p":
			if m.activeTab == tabFirearms {
				m.pdPickupChecked = !m.pdPickupChecked
				if !m.pdPickupChecked {
					m.inputs[m.activeTab][fieldPDPickupQty].SetValue("")
				}
				m.recalcActive()
			}
			return m, nil
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs[m.activeTab]))
	for i := range m.inputs[m.activeTab] {
		m.inputs[m.activeTab][i], cmds[i] = m.inputs[m.activeTab][i].Update(msg)
	}

	m.recalcActive()

	return m, tea.Batch(cmds...)
}

func checkbox(label string, checked bool) string {
	if checked {
		return fmt.Sprintf("[x] %s", label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	labelStyle := lipgloss.NewStyle().
		Width(30).
		Foreground(lipgloss.Color("69"))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		MarginRight(2)

	resultStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Padding(1, 2)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	tabActiveStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 2).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("62"))

	tabInactiveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("236"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("220"))

	banner := `███████╗███████╗████████╗     ██████╗ █████╗ ██╗      ██████╗
██╔════╝██╔════╝╚══██╔══╝    ██╔════╝██╔══██╗██║     ██╔════╝
█████╗  █████╗     ██║       ██║     ███████║██║     ██║
██╔══╝  ██╔══╝     ██║       ██║     ██╔══██║██║     ██║
██║     ███████╗   ██║       ╚██████╗██║  ██║███████╗╚██████╗
╚═╝     ╚══════╝   ╚═╝        ╚═════╝╚═╝  ╚═╝╚══════╝ ╚═════╝`

	renderTab := func(t tab) string {
		if t == m.activeTab {
			return tabActiveStyle.Render(tabName(t))
		}
		return tabInactiveStyle.Render(tabName(t))
	}

	tabBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		renderTab(tabFirearms),
		renderTab(tabParts),
		renderTab(tabBarrels),
	)

	row := func(label, value string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			labelStyle.Render(label),
			inputStyle.Render(value),
		)
	}

	leftLines := []string{
		titleStyle.Render(banner),
		subtitleStyle.Render("FFL Firearm / Parts / Barrels Tax Calculator"),
		"",
		tabBar,
		"",
		row(itemLabel(m.activeTab), m.inputs[m.activeTab][fieldPrice].View()),
		row("Shipping / handling", m.inputs[m.activeTab][fieldShipping].View()),
		row("Sales tax on invoice", m.inputs[m.activeTab][fieldSalesTaxPaid].View()),
	}

	if m.activeTab == tabFirearms {
		leftLines = append(leftLines,
			row("FET on invoice", m.inputs[m.activeTab][fieldFETPaid].View()),
		)
	}

	leftLines = append(leftLines,
		row(
			"Additional fees on invoice",
			m.inputs[m.activeTab][fieldAdditionalFees].View(),
		),
		row("Total paid", m.inputs[m.activeTab][fieldTotalPaid].View()),
		"",
	)

	if m.activeTab == tabFirearms {
		leftLines = append(leftLines,
			sectionStyle.Render("Firearm Service Fees"),
			checkbox("FFL Xfer (press f)", m.fflXferChecked),
		)

		if m.fflXferChecked {
			leftLines = append(leftLines,
				row("FFL Xfer qty", m.inputs[m.activeTab][fieldFFLXferQty].View()),
				"FFL Xfer fee = $150 per firearm",
			)
		}

		leftLines = append(leftLines,
			"",
			checkbox("PD Pickup (press p)", m.pdPickupChecked),
		)

		if m.pdPickupChecked {
			leftLines = append(leftLines,
				row("PD Pickup qty", m.inputs[m.activeTab][fieldPDPickupQty].View()),
				"PD Pickup fee = $80 pickup + $10 paperwork per firearm",
			)
		}

		leftLines = append(leftLines, "")
	}

	if m.activeTab == tabParts {
		leftLines = append(leftLines,
			sectionStyle.Render("Parts Handling Fees"),
			row(
				"Packages / invoices qty",
				m.inputs[m.activeTab][fieldPackageQty].View(),
			),
			"$25 per package / invoice received",
			"",
		)
	}

	if m.activeTab == tabBarrels {
		leftLines = append(leftLines,
			sectionStyle.Render("Barrels Handling Fees"),
			row(
				"Packages / invoices qty",
				m.inputs[m.activeTab][fieldPackageQty].View(),
			),
			"$50 per package / invoice received",
			"",
		)
	}

	leftLines = append(leftLines,
		subtitleStyle.Render("Rules"),
		"Sales tax = item + additional fees",
	)

	if fetApplies(m.activeTab) {
		leftLines = append(leftLines,
			"FET = item + shipping/handling + additional fees",
		)
	} else if m.activeTab == tabParts {
		leftLines = append(leftLines, "No FET on parts")
	} else {
		leftLines = append(leftLines, "No FET on barrels")
	}

	leftLines = append(leftLines,
		"",
		subtitleStyle.Render("Rates"),
		"Sales tax rate: 8.75%",
		"FET rate: 11%",
		"",
		helpStyle.Render(
			"Left/Right: switch tab • Tab/Shift+Tab: move • Enter: calculate • Esc: clear • q: quit",
		),
	)

	if m.activeTab == tabFirearms {
		leftLines = append(leftLines,
			helpStyle.Render("f: toggle FFL Xfer • p: toggle PD Pickup"),
		)
	}

	rightLines := []string{
		titleStyle.Render("Results"),
		"",
		fmt.Sprintf("Sales tax base:         %s",
			money(m.results[m.activeTab].SalesTaxBase)),
		fmt.Sprintf("Expected sales tax:     %s",
			money(m.results[m.activeTab].ExpectedSalesTax)),
	}

	if fetApplies(m.activeTab) {
		rightLines = append(rightLines,
			fmt.Sprintf("FET base:               %s",
				money(m.results[m.activeTab].FETBase)),
			fmt.Sprintf("Expected FET:           %s",
				money(m.results[m.activeTab].ExpectedFET)),
		)
	} else if m.activeTab == tabParts {
		rightLines = append(rightLines, "REMINDER:           No FET on parts")
	} else {
		rightLines = append(rightLines, "REMINDER:           No FET on barrels")
	}

	rightLines = append(rightLines,
		fmt.Sprintf("Expected invoice total: %s",
			money(m.results[m.activeTab].ExpectedTotal)),
		fmt.Sprintf("Invoice discrepancy:    %s",
			signedMoney(m.results[m.activeTab].InvoiceDiscrepancy)),
		"",
		sectionStyle.Render("Amount Due"),
		fmt.Sprintf("Sales tax due:          %s",
			money(m.results[m.activeTab].SalesTaxDue)),
	)

	if fetApplies(m.activeTab) {
		rightLines = append(rightLines,
			fmt.Sprintf("FET due:                %s",
				money(m.results[m.activeTab].FETDue)),
		)
	}

	rightLines = append(rightLines,
		fmt.Sprintf("Tax total due:          %s",
			money(m.results[m.activeTab].TotalDue)),
		fmt.Sprintf("Service / handling fees:%s",
			strings.Repeat(" ", 1)+money(m.results[m.activeTab].ServiceFees)),
		fmt.Sprintf("Grand total due:        %s",
			money(m.results[m.activeTab].GrandTotalDue)),
	)

	if m.results[m.activeTab].Error != "" {
		rightLines = append(
			rightLines,
			"",
			errorStyle.Render(m.results[m.activeTab].Error),
		)
	}

	left := strings.Join(leftLines, "\n")
	right := strings.Join(rightLines, "\n")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Render(left),
		resultStyle.Render(right),
	) + "\n"
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
