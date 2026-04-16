package wizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

type Prompter struct {
	reader *bufio.Reader
	writer io.Writer
	err    error
}

func NewPrompter(w io.Writer) *Prompter {
	return &Prompter{
		reader: bufio.NewReader(os.Stdin),
		writer: w,
	}
}

func (p *Prompter) Err() error {
	return p.err
}

func (p *Prompter) Line(prompt, defaultVal string) string {
	return p.LineWith(prompt, defaultVal, nil)
}

func (p *Prompter) LineWith(prompt, defaultVal string, validate func(string) string) string {
	if p.err != nil {
		return defaultVal
	}
	for {
		if defaultVal != "" {
			fmt.Fprintf(p.writer, "%s %s: ", promptLabelStyle.Render(prompt), helpStyle.Render("["+defaultVal+"]"))
		} else {
			fmt.Fprintf(p.writer, "%s: ", promptLabelStyle.Render(prompt))
		}
		line, err := p.reader.ReadString('\n')
		if err != nil {
			p.err = err
			return defaultVal
		}
		val := strings.TrimSpace(line)
		if val == "" {
			val = defaultVal
		}
		if val == "" {
			fmt.Fprintln(p.writer, warnTextStyle.Render("  此项必填，请输入。"))
			continue
		}
		if validate != nil {
			if msg := validate(val); msg != "" {
				fmt.Fprintf(p.writer, "%s\n", warnTextStyle.Render("  "+msg))
				continue
			}
		}
		return val
	}
}

func (p *Prompter) Password(prompt string) string {
	if p.err != nil {
		return ""
	}
	for {
		fmt.Fprintf(p.writer, "%s: ", promptLabelStyle.Render(prompt))
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(p.writer)
		if err != nil {
			p.err = err
			return ""
		}
		result := strings.TrimSpace(string(password))
		if result == "" {
			fmt.Fprintln(p.writer, warnTextStyle.Render("  此项必填，请输入。"))
			continue
		}
		return result
	}
}

// PasswordOptional reads a masked password; an empty input returns empty
// string so callers can treat it as "keep existing value".
func (p *Prompter) PasswordOptional(prompt string) string {
	if p.err != nil {
		return ""
	}
	fmt.Fprintf(p.writer, "%s %s: ", promptLabelStyle.Render(prompt), helpStyle.Render("（直接回车保留原值）"))
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(p.writer)
	if err != nil {
		p.err = err
		return ""
	}
	return strings.TrimSpace(string(password))
}

func (p *Prompter) Select(prompt string, options []string) int {
	if p.err != nil {
		return 0
	}
	fmt.Fprintln(p.writer, promptLabelStyle.Render(prompt))
	for i, opt := range options {
		fmt.Fprintf(p.writer, "  %s %s\n", optionIndexStyle.Render(fmt.Sprintf("%d)", i+1)), optionTextStyle.Render(opt))
	}
	for {
		fmt.Fprintf(p.writer, "%s ", promptLabelStyle.Render(fmt.Sprintf("请选择 [1-%d]:", len(options))))
		line, err := p.reader.ReadString('\n')
		if err != nil {
			p.err = err
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Fprintf(p.writer, "%s\n", warnTextStyle.Render(fmt.Sprintf("  请输入 1 到 %d 之间的数字。", len(options))))
	}
}

func (p *Prompter) Confirm(prompt string, defaultYes bool) bool {
	if p.err != nil {
		return false
	}
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Fprintf(p.writer, "%s %s: ", promptLabelStyle.Render(prompt), helpStyle.Render(hint))
	line, err := p.reader.ReadString('\n')
	if err != nil {
		p.err = err
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" {
		return defaultYes
	}
	return answer == "y" || answer == "yes"
}

func (p *Prompter) WaitEnter() {
	if p.err != nil {
		return
	}
	_, err := p.reader.ReadString('\n')
	if err != nil {
		p.err = err
	}
}

// OptionalLine reads a single line of input that may be left empty.
// Unlike Line/LineWith, an empty response is accepted without looping.
func (p *Prompter) OptionalLine(prompt string) string {
	if p.err != nil {
		return ""
	}
	fmt.Fprintf(p.writer, "%s: ", promptLabelStyle.Render(prompt))
	line, err := p.reader.ReadString('\n')
	if err != nil {
		p.err = err
		return ""
	}
	return strings.TrimSpace(line)
}

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
