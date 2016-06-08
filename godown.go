package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/davinche/godown/markdown"
	"github.com/davinche/godown/memory"
	"github.com/davinche/godown/server"
	"github.com/davinche/godown/subscribe"
	"github.com/davinche/godown/watcher"
	"github.com/kardianos/osext"
)

func main() {
	// Args
	port := flag.Int("p", 1337, "Port")
	browser := flag.String("b", "", "Specify the browser to preview with.")
	shouldLaunch := flag.Bool("l", false, "Open the preview in a browser.")
	flag.Parse()

	strPort := strconv.Itoa(*port)
	doneCh := make(chan struct{})

	if len(flag.Args()) < 1 {
		help()
		return
	}

	command := strings.ToLower(flag.Arg(0))
	if command != "start" && command != "stop" && command != "send" {
		help()
		return
	}

	// ------------------------------------------------------------------------
	// Parse the Command ------------------------------------------------------
	// ------------------------------------------------------------------------

	// stop command issued: send a stop request to the server
	if command == "stop" {
		fmt.Println("stopping")
		stop(flag.Arg(1), strPort)
		return
	}

	// Start and Send Commands require at least 1 positional argument
	if flag.Arg(1) == "" {
		help()
		return
	}

	// parse the html template
	binDir, err := osext.ExecutableFolder()
	if err != nil {
		fmt.Println("error: could not determine binary folder")
		os.Exit(1)
	}
	templates := template.Must(template.ParseFiles(binDir + "/index.html"))

	// ------------------------------------------------------------------------
	// Server Registration ----------------------------------------------------
	// ------------------------------------------------------------------------

	// Watcher
	watchMonitor := watcher.NewWatchMonitor()
	// Websocket
	socketServer := server.NewServer()
	socketServer.Register("/connect")
	// Static Files
	static := http.FileServer(http.Dir(binDir + "/static"))

	// API
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
			socketServer.CreateSubscriber(markdownFile)
			w.WriteHeader(http.StatusOK)
			return
		}

		fID := r.URL.Query().Get("id")
		// Add a file to memory?
		if r.Method == "PUT" {
			// read the body
			defer r.Body.Close()
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("error: could not read markdown body"))
				return
			}

			// create new inmemory markdown file
			memoryFile, err := memory.NewFile(fID, data)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("error: could not create new in-memory markdown file"))
				return
			}

			// create new registry
			socketServer.CreateSubscriber(memoryFile)
			w.WriteHeader(http.StatusOK)
			return
		}

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

	// ------------------------------------------------------------------------
	// Start Server Processes--------------------------------------------------
	// ------------------------------------------------------------------------
	fileChanges := make(chan *markdown.File)
	http.HandleFunc("/", serveRequest)
	http.Handle("/static/", http.StripPrefix("/static/", static))
	listenAndServe(strPort)
	go watchMonitor.Start(fileChanges)

	// START THE SERVER -------------------------------------------------------
	if command == "start" {
		markdownFile, err := markdown.NewFile(flag.Arg(1))
		if err != nil {
			fmt.Printf("error: could not obtain markdown file: %q\n", err)
			os.Exit(1)
			return
		}
		resp, err := addFile(strPort, markdownFile)
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Printf("error: could not add markdown file to the watch list: %q\n", err)
			os.Exit(1)
			return
		}

		if *shouldLaunch {
			launchBrowser(*browser, strPort, markdownFile)
		}
	}

	if command == "send" {
		id := flag.Arg(1)
		data := flag.Arg(2)
		if id == "" || data == "" {
			fmt.Println("identifier or data not specified")
			os.Exit(1)
			return
		}

		file, err := memory.NewFile(id, []byte(data))
		if err != nil {
			fmt.Printf("error: could not prepare data to send to server: %q\n", err)
			os.Exit(1)
			return
		}

		req, err := addData(strPort, id, file.Content)
		if err != nil || req.StatusCode != http.StatusOK {
			fmt.Printf("error: there was an error in the request to the server: e=%v; code=%v\n", err, req.StatusCode)
			os.Exit(1)
			return
		}

		if *shouldLaunch {
			launchBrowser(*browser, strPort, file)
		}
	}

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

// ----------------------------------------------------------------------------
// Helpers --------------------------------------------------------------------
// ----------------------------------------------------------------------------

func help() {
	fmt.Fprintln(os.Stdout, "usage: godown {FLAGS} [COMMANDS] <PATH>\n")
	fmt.Fprintln(os.Stdout, "  Watches a markdown file for changes and previews it in the browser.\n")
	fmt.Fprintln(os.Stdout, "FLAGS:\n")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stdout, "\nCOMMANDS:\n")
	fmt.Fprintln(os.Stdout, "  start <PATH>")
	fmt.Fprintf(os.Stdout, "        %s\n", "Starts watching a file at the given path.")
	fmt.Fprintln(os.Stdout, "  stop <PATH>")
	fmt.Fprintf(os.Stdout, "        %s\n", "*optional* path: "+
		"Stops watching a file if given a path or terminates the Godown daemon.")
	fmt.Fprintln(os.Stdout, "  send [ID] <Markdown Data>")
	fmt.Fprintf(os.Stdout, "        %s\n", "Given an identifier and markdown data, "+
		"send it to the daemon for rendering.")
}

// Send DELETE to server to either: stop watching a file or kill the server
func stop(filePath string, port string) {
	if filePath != "" {
		fmt.Printf("stopping file %q\n", filePath)
		markdownFile, err := markdown.NewFile(filePath)
		if err != nil {
			fmt.Printf("error: could not obtain markdown file: %q\n", err)
			return
		}
		if _, err = killWatcher(port, markdownFile.GetID()); err != nil {
			fmt.Printf("error: could not create stop file watcher: %q\n", err)
		}
		return
	}

	if _, err := killServer(port); err != nil {
		fmt.Printf("error: could not create stop server request: %q\n", err)
		return
	}
}

// Start the Markdown Daemon or Issue an "ADD FILE" request
func listenAndServe(port string) {
	// try to listen from the port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Printf("error: could not start server: %q\n", err)
		return
	}
	go http.Serve(listener, nil)
}

func launchBrowser(browser string, port string, file subscribe.Source) {
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
		args = append(args, "http://localhost:"+port+"?id="+file.GetID())
		command := exec.Command(args[0], args[1:]...)
		err := command.Start()
		if err != nil {
			fmt.Printf("error: could not open url: %v\n", err)
		}
	}
}

// ----------------------------------------------------------------------------
// HTTP API Helpers -----------------------------------------------------------
// ----------------------------------------------------------------------------
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

func addData(port string, id string, data []byte) (*http.Response, error) {
	client := http.Client{}
	req, err := http.NewRequest(
		"PUT",
		"http://localhost:"+port+"?id="+id,
		bytes.NewBuffer(data),
	)

	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

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
