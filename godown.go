package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"./server"
	"github.com/russross/blackfriday"
)

func render() {

}

func main() {
	// Args
	port := flag.Int("p", 1337, "GoDown Port")
	browser := flag.String("b", "", "Browser to preview with")
	doneCh := make(chan struct{})
	flag.Parse()
	strPort := strconv.Itoa(*port)

	if len(flag.Args()) < 1 {
		fmt.Println("Godown Command Required")
		return
	}

	command := strings.ToLower(flag.Arg(0))
	if command != "start" && command != "stop" {
		fmt.Println("Invalid Godown Command: (start | stop) required")
		return
	}

	// stop command issued: send a stop request to the server
	if command == "stop" {
		client := http.Client{}
		req, err := http.NewRequest("DELETE", "http://localhost:"+strPort, nil)
		if err != nil {
			fmt.Printf("GoDown Error: could not stop server %q\n", err)
			return
		}
		client.Do(req)
		return
	}

	file := flag.Arg(1)
	if file == "" {
		fmt.Println("File not specified")
		return
	}

	// Start the websocket server
	server := server.NewServer("/connect")
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
		fmt.Println("Cannot open browser")
	} else {
		args = append(args, "http://localhost:"+strPort)
		command := exec.Command(args[0], args[1:]...)
		err := command.Start()
		if err != nil {
			fmt.Printf("Error starting in browser: %v", err)
		}
	}

	// Try to watch the file
	stat, err := os.Stat(file)
	if err != nil {
		fmt.Printf("GoDown Error: Could not read file: %q\n", err)
		return
	}

	// render once
	fileStr, err := ioutil.ReadFile(file)
	if err == nil {
		renderedStr := blackfriday.MarkdownCommon(fileStr)
		server.Broadcast(string(renderedStr))
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
				fmt.Println("File change detected: reading file and rendering markdown")
				fileStr, err := ioutil.ReadFile(file)
				if err == nil {
					renderedStr := blackfriday.MarkdownCommon(fileStr)
					server.Broadcast(string(renderedStr))
					stat = newStat
					continue
				}
			}
		}
	}
}
