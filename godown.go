package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/davinche/godown/coordinator"
	"github.com/urfave/cli"
)

var port int
var browser string
var shouldLaunch bool

var logging string

func main() {
	// ------------------------------------------------------------------------
	// Flags ------------------------------------------------------------------
	// ------------------------------------------------------------------------
	app := cli.NewApp()
	app.Name = "godown"
	app.Usage = "A markdown previewer written in Go"
	app.Version = "0.1.1"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "port, p",
			Value:       1337,
			Usage:       "the port for the markdown server",
			Destination: &port,
		},
		cli.StringFlag{
			Name:        "browser, b",
			Value:       "",
			Usage:       "the browser to launch the markdown preview in",
			Destination: &browser,
		},
		cli.BoolFlag{
			Name:        "l",
			Usage:       "specify to launch automatically in the browser",
			Destination: &shouldLaunch,
		},
		cli.StringFlag{
			Name:        "logging",
			Usage:       "specify logging output (stdout, stderr)",
			Value:       "",
			Destination: &logging,
		},
	}

	// ------------------------------------------------------------------------
	// Commands ---------------------------------------------------------------
	// ------------------------------------------------------------------------

	app.Commands = []cli.Command{
		{
			Name:      "start",
			Usage:     "preview a file at a given path",
			ArgsUsage: "<FILEPATH>",
			Action:    start,
		},
		{
			Name:  "stop",
			Usage: "stops the markdown server",
		},
		{
			Name:  "send",
			Usage: "sends data from stdin to the markdown server",
		},
	}

	// See what kind of logging to do
	app.Before = func(c *cli.Context) error {
		switch strings.ToLower(logging) {
		case "stdout":
			log.SetOutput(os.Stdout)
		case "stderr":
			log.SetOutput(os.Stderr)
		default:
			log.SetOutput(ioutil.Discard)
		}
		return nil
	}
	app.Run(os.Args)
}

// ----------------------------------------------------------------------------
// Commands -------------------------------------------------------------------
// ----------------------------------------------------------------------------

func start(c *cli.Context) (ret error) {
	ret = nil
	file := c.Args().First()
	// Make sure a file to load is specified
	if file == "" {
		cli.ShowSubcommandHelp(c)
		return
	}

	log.Printf("start command: port=%d; shouldLaunch=%v, browser=%q; file=%q",
		port, shouldLaunch, browser, file)

	// See if we need to start the daemon
	coordinator, err := coordinator.New(port)
	if err == nil {
		go coordinator.Serve()
		addFile(file)
		if shouldLaunch {
			uniqueID := coordinator.GetID(file)
			if uniqueID != "" {
				launchBrowser(uniqueID)
			} else {
				log.Println("error: could not get uniqueID to launch browser")
			}
		}
		coordinator.Wait()
	}

	log.Printf("error: could not start coordinator: will try to POST: err=%q\n", err)
	addFile(file)
	return nil
}

// Launch Browser Helper
func launchBrowser(id string) {
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
		log.Fatalln("error: could not launch browser")
	}
	args = append(args, "http://localhost:"+strconv.Itoa(port)+"?id="+id)
	log.Printf("launch browser: args=%v", args)
	command := exec.Command(args[0], args[1:]...)
	err := command.Start()
	if err != nil {
		log.Printf("error: could not open url: %v\n", err)
	}
}

// ----------------------------------------------------------------------------
// HTTP API Helpers -----------------------------------------------------------
// ----------------------------------------------------------------------------
func addFile(filePath string) {
	client := http.Client{}
	marshalled, err := json.Marshal(&struct{ Path string }{filePath})
	if err != nil {
		log.Fatalf("error: could not marshal filePath: error=%q\n", err)
		return
	}
	req, err := http.NewRequest("POST", "http://localhost:"+strconv.Itoa(port), bytes.NewBuffer(marshalled))
	if err != nil {
		log.Fatalf("error: could create http request: error=%q\n", err)
		return
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not preview markdown file: err=%q; statusCode=%q\n", err, res.StatusCode)
		return
	}
}

// func addData(port string, id string, data []byte) (*http.Response, error) {
// 	client := http.Client{}
// 	req, err := http.NewRequest(
// 		"PUT",
// 		"http://localhost:"+port+"?id="+id,
// 		bytes.NewBuffer(data),
// 	)

// 	if err != nil {
// 		return nil, err
// 	}
// 	return client.Do(req)
// }

// func killServer(port string) (*http.Response, error) {
// 	client := http.Client{}
// 	req, err := http.NewRequest("DELETE", "http://localhost:"+port, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return client.Do(req)
// }

// func killWatcher(port string, fileID string) (*http.Response, error) {
// 	client := http.Client{}
// 	req, err := http.NewRequest("DELETE", "http://localhost:"+port+"?id="+fileID, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return client.Do(req)
// }
