package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tidwall/jd"
)

var (
	usage = `
jd - JSON Interactive Editor
usage: jd path

examples:
       jd user.json           Open a file named 'user.json'
       cat user.json | jd     Read from stdin

for more info: https://github.com/tidwall/jd
`
)

func main() {
	var path string
	if len(os.Args) == 1 {
		path = "-"
	} else if len(os.Args) == 2 {
		if os.Args[1] == "-h" {
			fmt.Fprintf(os.Stdout, "%s\n", strings.TrimSpace(usage))
			return
		}
		path = os.Args[1]
	}
	if err := jd.Exec(path); err != nil {
		log.Fatal(err)
	}
}
