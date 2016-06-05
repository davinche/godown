package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/davinche/godown/markdown"
	"github.com/davinche/godown/server"
	"github.com/davinche/godown/watcher"
	"github.com/kardianos/osext"
)

// Helper to make an HTTP DEL request to kill the server
func killServer(port string) (*http.Response, error) {
	client := http.Client{}
	req, err := http.NewRequest("DELETE", "http://localhost:"+port, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func killWatcher(port string, fileID string) (*http.Response, error) {
	client := http.Client{}
	req, err := http.NewRequest("DELETE", "http://localhost:"+port+"?id="+fileID, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// Helper to add a new file to be watched on an existing server
func addFile(port string, f *markdown.File) (*http.Response, error) {
	watchRequest := watcher.WatchFileRequest{File: f.Path}
	marshalled, err := json.Marshal(watchRequest)
	if err != nil {
		return nil, err
	}

	client := http.Client{}
	req, err := http.NewRequest("POST", "http://localhost:"+port, bytes.NewBuffer(marshalled))
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func launchBrowser(browser string, port string, file *markdown.File) {
	// Launch the browser
	var args []string
	if browser == "" {
		switch runtime.GOOS {
		case "darwin":
			args = append(args, "open", "-g")
			break
		case "linux":
			args = append(args, "xdg-open")
			break
		case "windows":
			args = append(args, "cmd", "/C", "start", "/B")
			break
		}
	} else {
		args = append(args, browser)
	}

	if len(args) == 0 {
		fmt.Println("warning: no open command")
	} else {
		args = append(args, "http://localhost:"+port+"?id="+file.ID)
		command := exec.Command(args[0], args[1:]...)
		err := command.Start()
		if err != nil {
			fmt.Printf("error: could not open url: %v\n", err)
		}
	}
}

func listenAndServe(port string, f *markdown.File, done chan struct{}) {
	// try to listen from the port
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Printf("error: could not start server; trying to add to existing server: %q\n", err)
		req, err := addFile(port, f)
		if err != nil || req.StatusCode != http.StatusOK {
			fmt.Printf("error: could not register file to watch with server: %q\n", err)
		}
		done <- struct{}{}
	}
}

func help() {
	fmt.Fprintln(os.Stdout, "usage: godown {FLAGS} [COMMANDS] <PATH>\n")
	fmt.Fprintln(os.Stdout, "  Watches a markdown file for changes and previews it in the browser.\n")
	fmt.Fprintln(os.Stdout, "FLAGS:\n")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stdout, "\nCOMMANDS:\n")
	fmt.Fprintln(os.Stdout, "  start <PATH>")
	fmt.Fprintf(os.Stdout, "        %s\n", "Starts watching a file given a path.")
	fmt.Fprintln(os.Stdout, "  stop <PATH>")
	fmt.Fprintf(os.Stdout, "        %s\n", "*optional* path: Stops watching a file if a path is given, otherwise stops the Godown daemon.")
}

func main() {
	// Args
	port := flag.Int("p", 1337, "GoDown Port")
	browser := flag.String("b", "", "Browser to preview with")
	flag.Parse()
	strPort := strconv.Itoa(*port)
	doneCh := make(chan struct{})

	if len(flag.Args()) < 1 {
		help()
		return
	}

	command := strings.ToLower(flag.Arg(0))
	if command != "start" && command != "stop" {
		help()
		return
	}

	file := flag.Arg(1)
	// stop command issued: send a stop request to the server
	if command == "stop" {
		fmt.Println("stopping")
		if file != "" {
			markdownFile, err := markdown.NewFile(file)
			fmt.Println("stopping file " + markdownFile.Path)
			if err != nil {
				fmt.Printf("error: could not obtain markdown file: %q\n", err)
				return
			}
			if _, err = killWatcher(strPort, markdownFile.ID); err != nil {
				fmt.Printf("error: could not create stop file watcher: %q\n", err)
			}
			return
		}

		if _, err := killServer(strPort); err != nil {
			fmt.Printf("error: could not create stop server request: %q\n", err)
			return
		}
	}

	// start command
	if file == "" {
		help()
		return
	}

	markdownFile, err := markdown.NewFile(file)
	if err != nil {
		fmt.Printf("error: could not obtain markdown file: %q\n", err)
		return
	}

	// parse the html template
	binDir, err := osext.ExecutableFolder()
	if err != nil {
		fmt.Println("error: could not determine binary folder")
		os.Exit(1)
	}
	fmt.Println(binDir)
	templates := template.Must(template.ParseFiles(binDir + "/index.html"))

	// ------------------------------------------------------------------------
	// Server Registration ----------------------------------------------------
	// ------------------------------------------------------------------------

	// Watcher ----------------------------------------------------------------
	watchMonitor := watcher.NewWatchMonitor()
	// Websocket --------------------------------------------------------------
	socketServer := server.NewServer(markdownFile)
	socketServer.Register("/connect")
	// Static Files -----------------------------------------------------------
	static := http.FileServer(http.Dir(binDir + "/static"))
	// API --------------------------------------------------------------------
	serveRequest := func(w http.ResponseWriter, r *http.Request) {
		// watch another file
		if r.Method == "POST" {
			defer r.Body.Close()
			decoded := watcher.WatchFileRequest{}
			decoder := json.NewDecoder(r.Body)
			decoder.Decode(&decoded)
			if decoded.File == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			markdownFile, err := markdown.NewFile(decoded.File)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("error: could not create new markdown file type"))
				return
			}

			// create a new watcher
			watchMonitor.AddWatcher(markdownFile)
			socketServer.CreateFileSubscriber(markdownFile)
			w.WriteHeader(http.StatusOK)
			return
		}

		fID := r.URL.Query().Get("id")
		// shutting down the server?
		if r.Method == "DELETE" {
			// are we removing one file?
			if fID != "" {
				go watchMonitor.RemoveWatcher(fID)
				w.WriteHeader(http.StatusOK)
				return
			}

			// or shut down the server
			w.WriteHeader(http.StatusOK)
			doneCh <- struct{}{}
			return
		}

		// retrieiving which markdown file to get
		if fID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// serving the file
		var host string
		if host, _, err = net.SplitHostPort(r.Host); err != nil {
			host = "localhost"
		}
		tStruct := struct {
			Host   string
			Port   string
			FileID string
		}{host, strPort, fID}
		templates.ExecuteTemplate(w, "index.html", tStruct)
	}

	fileChanges := make(chan *markdown.File)
	watchMonitor.AddWatcher(markdownFile)
	go watchMonitor.Start(fileChanges)

	http.Handle("/static/", http.StripPrefix("/static/", static))
	http.HandleFunc("/", serveRequest)
	go listenAndServe(strPort, markdownFile, doneCh)
	launchBrowser(*browser, strPort, markdownFile)

	// Loop message from watcher
	for {
		select {
		case <-doneCh:
			watchMonitor.Done()
			return
		case changes := <-fileChanges:
			socketServer.Broadcast(changes)
		}
	}
}
