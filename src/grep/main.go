package main

import (
	"bufio"
	"container/ring"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"code.google.com/p/getopt"
	"github.com/fatih/color"
)

var (
	isStdin   = false
	printSync = &sync.Mutex{}
	opts      = &Options{}

	hl = color.New(color.FgRed).SprintfFunc()
)

func init() {
	// initial value of NO color, like grep
	color.NoColor = true

	getopt.BoolVarLong(&opts.ShowHelp, "help", 'p', "show help information and usage")
	getopt.BoolVarLong(&opts.ShowVersion, "version", 'V', "show version information")

	getopt.BoolVarLong(&opts.UseRegex, "regexp", 'e', "match pattern as regexp")
	getopt.BoolVarLong(&opts.IgnoreCase, "ignore-case", 'i', "ignore case when searching")
	getopt.BoolVarLong(&opts.ListFiles, "files-with-matches", 'l', "only list files, not content")
	getopt.BoolVarLong(&opts.Color, "color", 'c', "colorize output")
	getopt.BoolVarLong(&opts.NoFileName, "no-filename", 'h', "don't output filenames")
	getopt.BoolVarLong(&opts.FileName, "filename", 'H', "output filenames (default if more than one file)")
	getopt.BoolVarLong(&opts.LineNums, "line-number", 'n', "show line numbers")
	getopt.BoolVarLong(&opts.InvertMatch, "invert-match", 'v', "invert the sense of matching, to select non-matching lines")

	getopt.IntVarLong(&opts.Context, "context", 'C', "show N lines of context on each side")
	getopt.IntVarLong(&opts.BeforeContext, "before", 'B', "show N lines of context before matches")
	getopt.IntVarLong(&opts.AfterContext, "after", 'A', "show N lines of context after matches")
}

func main() {
	pattern, files := parseArgs()

	wg := &sync.WaitGroup{}
	wg.Add(len(files))

	for _, file := range files {
		go func(f *os.File) {
			processFile(f, pattern)
			wg.Done()
		}(file)
	}

	wg.Wait()
}

func processFile(file *os.File, pattern string) {
	matches := make(chan *Match)

	go grepFile(file, pattern, matches)

	if opts.ListFiles {
		// if a match is returned then print the file name and move on
		if <-matches != nil {
			printSync.Lock()
			fmt.Println(file)
			printSync.Unlock()
		}
		// channel was closed without any results so there is no match
		return
	}

	output := ""
	fname := file.Name()
	for match := range matches {
		for _, l := range match.LinesBefore {
			if l != nil {
				output += lineFmt(fname, *l, "")
			}
		}

		output += lineFmt(fname, *match.Line, match.MatchStr)

		for _, l := range match.LinesAfter {
			if l != nil {
				output += lineFmt(fname, *l, "")
			}
		}
		if opts.BeforeContext+opts.AfterContext > 0 {
			output += "--\n--\n"
		}
	}
	output = strings.TrimRight(output, "--\n--\n")
	if output != "" {
		printSync.Lock()
		fmt.Println(output)
		printSync.Unlock()
	}
}

func lineFmt(fname string, l fileLine, matchStr string) string {
	var parts []string
	if !opts.NoFileName {
		parts = append(parts, fname)
	}
	if opts.LineNums {
		parts = append(parts, fmt.Sprintf("%d", l.Num))
	}

	sep := ":"
	if matchStr == "" {
		sep = "-"
	}
	output := strings.Join(parts, sep) + sep

	if opts.OnlyMatching {
		output += matchStr
	} else {
		if opts.Color {
			l.Text = strings.Replace(l.Text, matchStr, hl(matchStr), -1)
		}
		output += fmt.Sprintf("%s", l.Text)
	}
	return output + "\n"
}

func parseArgs() (pattern string, files []*os.File) {
	getopt.Parse()
	args := getopt.Args()

	if opts.ShowVersion {
		fmt.Println("grep-go")
		fmt.Println("version: 1.0")
		fmt.Println("author: Brett Jones")
		fmt.Println("source: https://github.com/blockloop/shell-go")
		os.Exit(0)
	}

	if opts.ShowHelp {
		getopt.Usage()
		os.Exit(0)
	}
	if len(args) == 0 {
		getopt.Usage()
		os.Exit(0)
	}

	if opts.Color {
		color.NoColor = false
	}
	if len(files) == 1 && !opts.FileName {
		opts.NoFileName = true
	}

	// parse pattern and file from remaining arguments
	if len(args) == 1 {
		isStdin = true
		files = append(files, os.Stdin)
	} else {
		for _, f := range args[1:] {
			if fl, err := os.Open(f); err == nil {
				files = append(files, fl)
			} else {
				fmt.Println("warning:", err.Error())
			}
		}
	}

	// this makes things easier later
	if opts.Context > 0 {
		opts.BeforeContext = opts.Context
		opts.AfterContext = opts.Context
	}
	return args[0], files
}

func grepFile(file *os.File, pattern string, to chan<- *Match) {
	if opts.UseRegex && opts.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	lines := make(chan *contextualLine)

	go readContextualFile(file, lines)

	for line := range lines {
		if line == nil || line.Current == nil {
			continue
		}

		match := findMatch(line.Current.Text, pattern)

		// XOR - either Invert or it is a match, but not both
		if opts.InvertMatch != (match != "") {
			to <- &Match{
				MatchStr:    match,
				LinesBefore: line.LinesBefore,
				LinesAfter:  line.LinesAfter,
				Line: &fileLine{
					Text: line.Current.Text,
					Num:  line.Current.Num,
				},
			}
		}
	}

	close(to)
}

func findMatch(s string, pattern string) (match string) {
	if opts.UseRegex {
		r := regexp.MustCompile(pattern)
		finds := r.FindAllString(s, -1)
		if finds != nil {
			return finds[0]
		}
	} else {
		if opts.IgnoreCase && strings.Contains(strings.ToLower(s), strings.ToLower(pattern)) {
			return pattern
		} else if strings.Contains(s, pattern) {
			return pattern
		}
	}
	return ""
}

type contextualLine struct {
	LinesBefore []*fileLine
	LinesAfter  []*fileLine
	Current     *fileLine
}

func readContextualFile(file *os.File, to chan<- *contextualLine) {
	// ring to hold buffer before and after and current line
	totalContext := opts.BeforeContext + opts.AfterContext
	buffer := ring.New(totalContext + 1)

	lineChan := make(chan *fileLine)
	go func() {
		readFile(file, lineChan)
		// when the file is finished being read the last N lines will remain in the AFTER position
		// so we push nils into the channel to move the last lines through the current line
		for i := 0; i < totalContext; i++ {
			lineChan <- nil
		}
		close(lineChan)
	}()

	for line := range lineChan {
		res := &contextualLine{}

		buffer.Value = line
		buffer = buffer.Next()

		if line != nil && line.Num <= totalContext {
			// don't have enough buffer, wait for more
			continue
		}

		// start with -1 because i++ is used at the beginning of the Do loop so it can't be missed
		i := -1
		buffer.Do(func(b interface{}) {
			i++
			var fl *fileLine
			if b != nil {
				fl = b.(*fileLine)
			}
			if i < opts.BeforeContext {
				res.LinesBefore = append(res.LinesBefore, fl)
				return
			}
			if i == opts.BeforeContext {
				res.Current = fl
				return
			}
			res.LinesAfter = append(res.LinesAfter, fl)
		})

		to <- res
	}
	close(to)
}

func readFile(file *os.File, to chan<- *fileLine) {
	defer file.Close()

	freader := bufio.NewReader(file)
	for i := 1; ; i++ {
		line, _, er := freader.ReadLine()
		if er != nil {
			break
		}
		to <- &fileLine{Num: i, Text: string(line)}
	}
}

// Options from the command line
type Options struct {
	Color         bool
	ShowHelp      bool
	ShowVersion   bool
	UseRegex      bool
	IgnoreCase    bool
	InvertMatch   bool
	ListFiles     bool
	LineNums      bool
	OnlyMatching  bool
	Context       int
	AfterContext  int
	BeforeContext int
	NoFileName    bool
	FileName      bool
}

type fileLine struct {
	Text string
	Num  int
}

// Match is a matching line from a file
type Match struct {
	Line        *fileLine
	MatchStr    string
	LinesBefore []*fileLine
	LinesAfter  []*fileLine
}
