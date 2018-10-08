package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
var VERSION string

func main() {
	// ------------------------------------------------------------------------
	// Flags ------------------------------------------------------------------
	// ------------------------------------------------------------------------
	app := cli.NewApp()
	app.Name = "godown"
	app.Usage = "A markdown previewer written in Go"
	app.Version = VERSION
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
			Name:      "stop",
			Usage:     "stops the markdown server",
			ArgsUsage: "<FILEPATH>",
			Action:    stop,
		},
		{
			Name:      "send",
			Usage:     "sends data from stdin to the markdown server",
			ArgsUsage: "<ID>",
			Action:    send,
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
			log.SetFlags(0)
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
		// start the daemon
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
		return
	}

	log.Printf("error: could not start coordinator: will try to POST: err=%q\n", err)
	addFile(file)
	if shouldLaunch {
		uniqueID := getID(file)
		if uniqueID != "" {
			launchBrowser(uniqueID)
		}
	}
	return
}

func stop(c *cli.Context) (ret error) {
	ret = nil
	file := c.Args().First()
	log.Printf("stop command: port=%d; file=%q\n", port, file)
	if file == "" {
		killServer()
		return
	}
	killFile(file)
	return
}

func send(c *cli.Context) (ret error) {
	ret = nil
	file := c.Args().First()
	// Make sure an identifier is sent
	if file == "" {
		cli.ShowSubcommandHelp(c)
		return
	}
	log.Printf("send command: port=%d; shouldLaunch=%v\n", port, shouldLaunch)
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("error: could not read markdown data: error=%q\n", err)
	}
	log.Printf("send command: read data: data=%q\n", string(data))

	// See if we need to start the daemon
	coordinator, err := coordinator.New(port)
	if err == nil {
		// start the daemon
		go coordinator.Serve()
		addData(file, data)
		if shouldLaunch {
			fmt.Println("FILE")
			fmt.Println(file)
			uniqueID := coordinator.GetID(file)
			fmt.Println("UNIQUEID")
			fmt.Println(uniqueID)
			if uniqueID != "" {
				launchBrowser(uniqueID)
			} else {
				log.Println("error: could not get uniqueID to launch browser")
			}
		}
		coordinator.Wait()
		return
	}
	addData(file, data)
	if shouldLaunch {
		uniqueID := getID(file)
		if uniqueID != "" {
			launchBrowser(uniqueID)
		}
	}
	return
}

// ----------------------------------------------------------------------------
// Launch Browser Helper-------------------------------------------------------
// ----------------------------------------------------------------------------

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
		log.Println("error: could not determine how to launch browser")
	}
	args = append(args, "http://localhost:"+strconv.Itoa(port)+"/api?id="+id)
	log.Printf("launch browser cmd: args=%v\n", args)
	command := exec.Command(args[0], args[1:]...)
	err := command.Start()
	if err != nil {
		log.Printf("error: could not launch browser: %v\n", err)
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
	}
	req, err := http.NewRequest("POST", "http://localhost:"+strconv.Itoa(port)+"/api", bytes.NewBuffer(marshalled))
	if err != nil {
		log.Fatalf("error: could create http request: error=%q\n", err)
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not preview markdown file: err=%q; statusCode=%q\n", err, res.StatusCode)
	}
}

func getID(filePath string) string {
	client := http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:"+strconv.Itoa(port)+"/getid?path="+filePath, nil)
	if err != nil {
		log.Fatalf("error: could create http getID request: error=%q\n", err)
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not get ID of the file: err=%q; statusCode=%q\n", err, res.StatusCode)
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("error: could not get ID of the file from body: err=%q\n")
	}
	return string(data)
}

func addData(id string, data []byte) {
	fmt.Println("in ADD DAata")
	fmt.Println(id)
	client := http.Client{}
	req, err := http.NewRequest(
		"PUT",
		"http://localhost:"+strconv.Itoa(port)+"/api?id="+id,
		bytes.NewBuffer(data),
	)

	if err != nil {
		log.Fatalf("error: could not create PUT request: error=%q\n", err)
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not send data to markdown server: error=%q; statusCode=%q\n", err, res.StatusCode)
	}
}

func killServer() {
	client := http.Client{}
	req, err := http.NewRequest("DELETE", "http://localhost:"+strconv.Itoa(port)+"/api", nil)
	if err != nil {
		log.Fatalf("error: could not create shutdown request: error=%q\n", err)
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not shutdown server: error=%q; statusCode=%q\n", err, res.StatusCode)
	}
}

func killFile(file string) {
	client := http.Client{}
	req, err := http.NewRequest("DELETE", "http://localhost:"+strconv.Itoa(port)+"/api?id="+file, nil)
	if err != nil {
		log.Fatalf("error: could not create delete file request: error=%q\n", err)
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		log.Fatalf("error: could not delete file: error=%q; statusCode=%q\n", err, res.StatusCode)
	}
}
