// Package prompter is utility for easy prompting
package prompter

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/ssh/terminal"
)

// VERSION version of prompter
const VERSION = "0.1.0"

// Prompter is object for prompting
type Prompter struct {
	Message string
	// choices of answer
	Choices    []string
	IgnoreCase bool
	Default    string
	// specify answer pattern by regexp. When both Choices and Regexp are specified, Regexp takes a priority.
	Regexp *regexp.Regexp
	// for passwords and so on.
	NoEcho     bool
	UseDefault bool
	reg        *regexp.Regexp
}

// Prompt displays a prompt and returns answer
func (p *Prompter) Prompt() string {
	fmt.Print(p.msg())
	if p.UseDefault || skip() {
		return p.Default
	}

	input := ""
	for {
		if p.NoEcho {
			b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
			if err == nil {
				input = string(b)
			}
			fmt.Print("\n")
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			ok := scanner.Scan()
			if ok {
				input = strings.TrimRight(scanner.Text(), "\r\n")
			}
		}
		if input == "" {
			input = p.Default
		}
		if p.inputIsValid(input) {
			break
		}
		fmt.Println(p.errorMsg())
		fmt.Print(p.msg())
	}
	return input
}

func skip() bool {
	if os.Getenv("GO_PROMPTER_USE_DEFAULT") != "" {
		return true
	}
	return !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd())
}

func (p *Prompter) msg() string {
	msg := p.Message
	if p.Choices != nil && len(p.Choices) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(p.Choices, "/"))
	}
	if p.Default != "" {
		msg += " [" + p.Default + "]"
	}
	return msg + ": "
}

func (p *Prompter) errorMsg() string {
	if p.Regexp != nil {
		return fmt.Sprintf("# Answer should match /%s/", p.Regexp)
	}
	if p.Choices != nil && len(p.Choices) > 0 {
		if len(p.Choices) == 1 {
			return fmt.Sprintf("# Enter `%s`", p.Choices[0])
		}
		choices := make([]string, len(p.Choices)-1)
		for i, v := range p.Choices[:len(p.Choices)-1] {
			choices[i] = "`" + v + "`"
		}
		return fmt.Sprintf("# Enter %s or `%s`", strings.Join(choices, ", "), p.Choices[len(p.Choices)-1])
	}
	return ""
}

func (p *Prompter) inputIsValid(input string) bool {
	return p.regexp().MatchString(input)
}

var allReg = regexp.MustCompile(`.*`)

func (p *Prompter) regexp() *regexp.Regexp {
	if p.Regexp != nil {
		return p.Regexp
	}
	if p.reg != nil {
		return p.reg
	}
	if p.Choices == nil || len(p.Choices) == 0 {
		p.reg = allReg
		return p.reg
	}

	choices := make([]string, len(p.Choices))
	for i, v := range p.Choices {
		choices[i] = regexp.QuoteMeta(v)
	}
	ignoreReg := ""
	if p.IgnoreCase {
		ignoreReg = "(?i)"
	}
	p.reg = regexp.MustCompile(fmt.Sprintf(`%s\A(?:%s)\z`, ignoreReg, strings.Join(choices, "|")))
	return p.reg
}
