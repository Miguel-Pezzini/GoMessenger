package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type testEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

type testStatus struct {
	Name    string
	Status  string
	Elapsed float64
	Logs    []string
}

type packageStatus struct {
	Name         string
	Status       string
	Elapsed      float64
	Tests        map[string]*testStatus
	PackageLogs  []string
	PassedTests  int
	FailedTests  int
	SkippedTests int
}

type formatter struct {
	useColor        bool
	startedAt       time.Time
	lastEventAt     time.Time
	packages        map[string]*packageStatus
	passedPackages  int
	failedPackages  int
	skippedPackages int
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

func main() {
	f := &formatter{
		useColor: isTerminal(os.Stdout.Fd()) && strings.ToLower(os.Getenv("NO_COLOR")) == "",
		packages: make(map[string]*packageStatus),
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var event testEvent
		if err := json.Unmarshal(line, &event); err != nil {
			fmt.Println(string(line))
			continue
		}

		f.handleEvent(event)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read go test output: %v\n", err)
		os.Exit(1)
	}

	f.printSummary()

	if f.failedPackages > 0 || f.countFailedTests() > 0 {
		os.Exit(1)
	}
}

func (f *formatter) handleEvent(event testEvent) {
	if f.startedAt.IsZero() && !event.Time.IsZero() {
		f.startedAt = event.Time
	}
	if !event.Time.IsZero() {
		f.lastEventAt = event.Time
	}

	pkg := f.getPackage(event.Package)
	if event.Test != "" {
		test := f.getTest(pkg, event.Test)
		switch event.Action {
		case "output":
			if line := sanitizeOutput(event.Output); line != "" {
				test.Logs = append(test.Logs, line)
			}
		case "pass", "fail", "skip":
			test.Status = event.Action
			test.Elapsed = event.Elapsed
			f.printTestResult(pkg.Name, test)
			if event.Action == "pass" {
				pkg.PassedTests++
			}
			if event.Action == "fail" {
				pkg.FailedTests++
			}
			if event.Action == "skip" {
				pkg.SkippedTests++
			}
		}
		return
	}

	switch event.Action {
	case "output":
		if line := sanitizeOutput(event.Output); line != "" {
			pkg.PackageLogs = append(pkg.PackageLogs, line)
		}
	case "pass", "fail", "skip":
		pkg.Status = event.Action
		pkg.Elapsed = event.Elapsed
		f.printPackageResult(pkg)
		if event.Action == "pass" {
			f.passedPackages++
		}
		if event.Action == "fail" {
			f.failedPackages++
			f.printPackageLogs(pkg)
		}
		if event.Action == "skip" {
			f.skippedPackages++
		}
	}
}

func (f *formatter) getPackage(name string) *packageStatus {
	if name == "" {
		name = "(unknown package)"
	}
	if pkg, ok := f.packages[name]; ok {
		return pkg
	}
	pkg := &packageStatus{
		Name:  name,
		Tests: make(map[string]*testStatus),
	}
	f.packages[name] = pkg
	return pkg
}

func (f *formatter) getTest(pkg *packageStatus, name string) *testStatus {
	if test, ok := pkg.Tests[name]; ok {
		return test
	}
	test := &testStatus{Name: name}
	pkg.Tests[name] = test
	return test
}

func (f *formatter) printTestResult(pkgName string, test *testStatus) {
	label, color := statusStyle(f.useColor, test.Status)
	fmt.Printf("%s %s %s %s\n", label, f.dim(shortPackage(pkgName)), colorize(f.useColor, color, test.Name), f.dim(formatElapsed(test.Elapsed)))
	if test.Status == "fail" {
		for _, logLine := range uniqueLogs(test.Logs) {
			fmt.Printf("  %s\n", f.dim(logLine))
		}
	}
}

func (f *formatter) printPackageResult(pkg *packageStatus) {
	label, color := statusStyle(f.useColor, pkg.Status)
	testCount := pkg.PassedTests + pkg.FailedTests + pkg.SkippedTests
	testWord := "tests"
	if testCount == 1 {
		testWord = "test"
	}
	fmt.Printf("%s %s %s %s %s\n", label, f.bold(shortPackage(pkg.Name)), colorize(f.useColor, color, strings.ToUpper(pkg.Status)), f.dim(fmt.Sprintf("(%d %s)", testCount, testWord)), f.dim(formatElapsed(pkg.Elapsed)))
}

func (f *formatter) printPackageLogs(pkg *packageStatus) {
	for _, line := range uniqueLogs(pkg.PackageLogs) {
		if strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "exit status ") {
			continue
		}
		fmt.Printf("  %s\n", f.dim(line))
	}
}

func (f *formatter) printSummary() {
	totalTests := 0
	passedTests := 0
	failedTests := 0
	skippedTests := 0
	for _, pkg := range f.packages {
		totalTests += pkg.PassedTests + pkg.FailedTests + pkg.SkippedTests
		passedTests += pkg.PassedTests
		failedTests += pkg.FailedTests
		skippedTests += pkg.SkippedTests
	}

	totalPackages := f.passedPackages + f.failedPackages + f.skippedPackages
	totalElapsed := 0.0
	if !f.startedAt.IsZero() && !f.lastEventAt.IsZero() {
		totalElapsed = f.lastEventAt.Sub(f.startedAt).Seconds()
	}

	fmt.Println()
	if failedTests > 0 || f.failedPackages > 0 {
		fmt.Println(colorize(f.useColor, colorRed, "FAIL"))
	} else {
		fmt.Println(colorize(f.useColor, colorGreen, "PASS"))
	}

	summaryParts := []string{
		fmt.Sprintf("%d packages", totalPackages),
		fmt.Sprintf("%d tests", totalTests),
		colorize(f.useColor, colorGreen, fmt.Sprintf("%d passed", passedTests)),
	}
	if skippedTests > 0 {
		summaryParts = append(summaryParts, colorize(f.useColor, colorYellow, fmt.Sprintf("%d skipped", skippedTests)))
	}
	if failedTests > 0 {
		summaryParts = append(summaryParts, colorize(f.useColor, colorRed, fmt.Sprintf("%d failed", failedTests)))
	}
	if totalElapsed > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("finished in %s", formatElapsed(totalElapsed)))
	}
	fmt.Println(strings.Join(summaryParts, f.dim("  •  ")))

	if totalPackages > 0 {
		packageLine := []string{
			colorize(f.useColor, colorGreen, fmt.Sprintf("%d passed", f.passedPackages)),
		}
		if f.skippedPackages > 0 {
			packageLine = append(packageLine, colorize(f.useColor, colorYellow, fmt.Sprintf("%d skipped", f.skippedPackages)))
		}
		if f.failedPackages > 0 {
			packageLine = append(packageLine, colorize(f.useColor, colorRed, fmt.Sprintf("%d failed", f.failedPackages)))
		}
		fmt.Printf("%s %s\n", f.dim("packages:"), strings.Join(packageLine, f.dim("  •  ")))
	}
}

func (f *formatter) countFailedTests() int {
	count := 0
	for _, pkg := range f.packages {
		count += pkg.FailedTests
	}
	return count
}

func statusStyle(useColor bool, status string) (string, string) {
	switch status {
	case "pass":
		return colorize(useColor, colorGreen, "✓"), colorGreen
	case "fail":
		return colorize(useColor, colorRed, "✗"), colorRed
	case "skip":
		return colorize(useColor, colorYellow, "○"), colorYellow
	default:
		return colorize(useColor, colorBlue, "•"), colorBlue
	}
}

func sanitizeOutput(output string) string {
	line := strings.TrimRight(output, "\r\n")
	line = strings.TrimPrefix(line, "=== RUN   ")
	line = strings.TrimPrefix(line, "=== PAUSE ")
	line = strings.TrimPrefix(line, "=== CONT  ")
	if strings.HasPrefix(line, "--- PASS:") || strings.HasPrefix(line, "--- FAIL:") || strings.HasPrefix(line, "--- SKIP:") {
		return ""
	}
	return strings.TrimSpace(line)
}

func uniqueLogs(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	unique := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		unique = append(unique, line)
	}
	return unique
}

func shortPackage(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) <= 2 {
		return pkg
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func formatElapsed(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	switch {
	case d >= time.Minute:
		return d.Round(10 * time.Millisecond).String()
	case d >= time.Second:
		return d.Round(time.Millisecond).String()
	case d >= time.Millisecond:
		return d.Round(time.Millisecond).String()
	default:
		return d.Round(time.Microsecond).String()
	}
}

func (f *formatter) bold(value string) string {
	if !f.useColor {
		return value
	}
	return "\033[1m" + value + colorReset
}

func (f *formatter) dim(value string) string {
	return colorize(f.useColor, colorGray, value)
}

func colorize(enabled bool, color string, value string) string {
	if !enabled || value == "" {
		return value
	}
	return color + value + colorReset
}

func isTerminal(fd uintptr) bool {
	file := os.NewFile(fd, "")
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
