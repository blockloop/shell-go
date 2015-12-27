package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/color"
)

var (
	cmd      = *kingpin.Command("")
	useRegex = *kingpin.Flag("regexp", "enable debug mode").Default("false").Bool()
	files    = *kingpin.Arg("files", "files to search").Strings()
	pat      = *kingpin.Arg("pattern", "pattern to search for").String()
	// serverIP = kingpin.Flag("server", "server address").Default("127.0.0.1").IP()
	// register     = kingpin.Command("register", "Register a new user.")
	// registerNick = register.Arg("nick", "nickname for user").Required().String()
	// registerName = register.Arg("name", "name of user").Required().String()
	// post        = kingpin.Command("post", "Post a message to a channel.")
	// postImage   = post.Flag("image", "image to post").ExistingFile()
	// postChannel = post.Arg("channel", "channel to post to").Required().String()
	// postText    = post.Arg("text", "text to post").String()

	yellow = color.New(color.FgBlack, color.BgYellow).SprintfFunc()
)

func init() {
}

func main() {
	// var opts options
	kingpin.Parse()
	kingpin.Usage()
	os.Exit(1)
	var wg sync.WaitGroup

	resultChan := make(chan string)

	wg.Add(len(files))
	for _, file := range files {
		go func(f string) {
			grepFile(f, pat, resultChan)
			wg.Done()
		}(file)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for res := range resultChan {
		println(res)
	}

}

// func makeFlags() {
// 	flags.
// }

// func parseArgs() (files []string, pat string) {
// if len(os.Args) < 3 {
// 	println("usage: grep [-abcDEFGHhIiJLlmnOoqRSsUVvwxZ] [-A num] [-B num] [-C[num]]")
// 	println("        [-e pattern] [-f file] [--binary-files=value] [--color=when]")
// 	println("        [--context[=num]] [--directories=action] [--label] [--line-buffered]")
// 	println("        [--null] [pattern] [file ...]")
// 	os.Exit(1)
// }

// pat = os.Args[1]
// files = os.Args[2:]
// return files, pat
// }

func getFiles(files []string, to chan<- string) {
}

func grepFile(fyle string, pat string, to chan<- string) {
	var foundLines = ""
	linesChan := make(chan *Line)
	go readFile(fyle, linesChan)

	for line := range linesChan {
		if useRegex && regexp.MustCompile(pat).MatchString(line.Text) {

		} else if strings.Contains(line.Text, pat) {
			num := color.YellowString(strconv.Itoa(line.Num))
			text := strings.Replace(line.Text, pat, yellow(pat), -1)
			foundLines += fmt.Sprintf("    %s: %s", num, text)
		}
	}

	if foundLines != "" {
		fileColor := color.CyanString(fyle)
		to <- fmt.Sprintf("%s: \n%s\n", fileColor, foundLines)
	}
}

func readFile(file string, to chan<- *Line) {
	f, err := os.Open(file)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	freader := bufio.NewReader(f)
	i := 1
	for {
		line, er := freader.ReadBytes('\n')
		if er == nil {
			to <- &Line{Num: i, Text: string(line)}
		} else {
			break
		}
		i++
	}
	close(to)
}

// func grepLine(pat string, from <-chan string, result chan<- bool) {
// 	var wg sync.WaitGroup

// 	for line := range from {
// 		wg.Add(1)

// 		go func(l string) {
// 			defer wg.Done()
// 			if strings.Contains(l, pat) {
// 				result <- true
// 			}
// 		}(string(line))
// 	}

// 	wg.Wait()
// 	close(result)
// }

type options struct {
}

type Line struct {
	Num  int
	Text string
}
