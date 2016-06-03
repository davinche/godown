package main

import (
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
	"time"

	"github.com/davinche/godown/server"
	"github.com/kardianos/osext"
)

func killServer(port string) (*http.Response, error) {
	client := http.Client{}
	req, err := http.NewRequest("DELETE", "http://localhost:"+port, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func main() {
	// Args
	port := flag.Int("p", 1337, "GoDown Port")
	browser := flag.String("b", "", "Browser to preview with")
	flag.Parse()
	strPort := strconv.Itoa(*port)
	doneCh := make(chan struct{})

	help := func() {
		fmt.Fprintln(os.Stdout, "usage: godown {FLAGS} [COMMANDS] <PATH>\n")
		fmt.Fprintln(os.Stdout, "  Watches changes to a file and previews the markdown in the browser\n")
		fmt.Fprintln(os.Stdout, "FLAGS:\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stdout, "\nCOMMANDS:\n")
		fmt.Fprintf(os.Stdout, "  %-15s%s", "start PATH", "starts watching a file given a path\n")
		fmt.Fprintf(os.Stdout, "  %-15s%s", "stop", "stops the GoDown process\n")
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
		if _, err := killServer(strPort); err != nil {
			fmt.Printf("error: could not create stop server request: %q\n", err)
			return
		}
	}

	file := flag.Arg(1)
	if file == "" {
		help()
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

	// Start the websocket server
	server := server.NewServer("/connect", file)
	static := http.FileServer(http.Dir(binDir + "/static"))
	serveRequest := func(w http.ResponseWriter, r *http.Request) {
		var host string
		if host, _, err = net.SplitHostPort(r.Host); err != nil {
			host = "localhost"
		}
		tStruct := struct {
			Host string
			Port string
		}{host, strPort}
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			doneCh <- struct{}{}
		} else {

			templates.ExecuteTemplate(w, "index.html", tStruct)
		}
	}
	http.Handle("/static/", http.StripPrefix("/static/", static))
	http.HandleFunc("/", serveRequest)
	go server.Listen()
	go func() {
		// try to listen from the port
		err := http.ListenAndServe(":"+strPort, nil)
		if err != nil {
			fmt.Printf("error: could not start server, will retry after kill: %q\n", err)
			resp, err := killServer(strPort)
			if err != nil {
				fmt.Printf("error: could not start server: %q\n", err)
				os.Exit(1)
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("error: could not stop existing application on port %q\n", strPort)
				os.Exit(1)
			}
		}
		<-time.After(time.Millisecond * 100)
		fmt.Printf("error: could not start server: %q", http.ListenAndServe(":"+strPort, nil))
		os.Exit(1)
	}()

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
