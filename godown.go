package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/davinche/godown/server"
)

func main() {
	// Args
	port := flag.Int("p", 1337, "GoDown Port")
	browser := flag.String("b", "", "Browser to preview with")
	doneCh := make(chan struct{})
	flag.Parse()
	strPort := strconv.Itoa(*port)

	help := func() {
		fmt.Fprintln(os.Stdout, "usage: godown {COMMANDS} <PATH> {FLAGS}\n")
		fmt.Fprintln(os.Stdout, "  Watches changes to a file and previews the markdown in the browser\n")
		fmt.Fprintln(os.Stdout, "COMMANDS:\n")
		fmt.Fprintf(os.Stdout, "  %-15s%s", "start PATH", "starts watching a file given a path\n")
		fmt.Fprintf(os.Stdout, "  %-15s%s", "stop", "stops the GoDown process\n\n")
		fmt.Fprintln(os.Stdout, "FLAGS:")
		flag.PrintDefaults()
	}

	if len(flag.Args()) < 1 {
		help()
		return
	}

	command := strings.ToLower(flag.Arg(0))
	if command != "start" && command != "stop" {
		help()
		return
	}

	// stop command issued: send a stop request to the server
	if command == "stop" {
		client := http.Client{}
		req, err := http.NewRequest("DELETE", "http://localhost:"+strPort, nil)
		if err != nil {
			fmt.Printf("error: could not create stop server request: %q\n", err)
			return
		}
		client.Do(req)
		return
	}

	file := flag.Arg(1)
	if file == "" {
		help()
		return
	}

	// Start the websocket server
	server := server.NewServer("/connect", file)
	static := http.FileServer(http.Dir("./static"))
	serveRequest := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			doneCh <- struct{}{}
		} else {
			static.ServeHTTP(w, r)
		}
	}
	http.HandleFunc("/", serveRequest)
	go server.Listen()
	go http.ListenAndServe(":"+strPort, nil)

	// Launch the browser
	var args []string
	if *browser == "" {
		switch runtime.GOOS {
		case "darwin":
			args = append(args, "open", "-g")
			break
		case "linux":
			args = append(args, "xdg-open")
			break
		case "windows":
			args = append(args, "start")
			break
		}
	} else {
		args = append(args, *browser)
	}

	if len(args) == 0 {
		fmt.Println("warning: no open command")
	} else {
		args = append(args, "http://localhost:"+strPort)
		command := exec.Command(args[0], args[1:]...)
		err := command.Start()
		if err != nil {
			fmt.Printf("error: could not open url: %v", err)
		}
	}

	// Try to watch the file
	stat, err := os.Stat(file)
	if err != nil {
		fmt.Printf("error: could not read file: %q\n", err)
		return
	}

	// Loop watch
	for {
		select {
		case <-doneCh:
			server.Done()
			<-server.HasShutdown
			return
		case <-time.After(time.Second):
			newStat, err := os.Stat(file)
			if err != nil {
				continue
			}
			// something changed
			if newStat.Size() != stat.Size() || newStat.ModTime() != stat.ModTime() {
				server.Broadcast()
				stat = newStat
			}
		}
	}
}
