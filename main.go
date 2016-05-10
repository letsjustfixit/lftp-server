package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Request represents single request for mirroring one FTP directory or a file.
type Request struct {
	Path     string `json:"path"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Handler implements http.Handler interface and logs errors to custom log.Logger.
type Handler struct {
	Logger *log.Logger
	Jobs   chan *Job
}

// Job is single download request with associated ID.
type Job struct {
	ID      string
	Request *Request
	*exec.Cmd
}

func (request *Request) makeCmd() (*exec.Cmd, error) {
	if request.Path == "" {
		return nil, errors.New("No URL specified in a request")
	}

	url, err := url.Parse(request.Path)

	if err != nil {
		return nil, fmt.Errorf("Invalid URL: %s", request.Path)
	}

	lftpCmd := makeLftpCmd(url.Path)
	var args []string

	if request.Username != "" && request.Password != "" {
		args = []string{"--user", request.Username, "--password", request.Password, "-e", lftpCmd, url.Host}
	} else {
		args = []string{"-e", lftpCmd, url.Host}
	}

	cmd := exec.Command("lftp", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func generateID() string {
	b := make([]byte, 256)

	if _, err := rand.Read(b); err != nil {
		panic("Random number generator failed")
	}

	return base64.StdEncoding.EncodeToString(b)
}

func makeLftpCmd(path string) string {
	if path == "" {
		return "mirror && exit"
	}

	escaped := strings.Replace(path, "\"", "\\\"", -1)
	return fmt.Sprintf("mirror \"%s\" && exit", escaped)
}

func decodeRequest(r io.Reader) (*Request, error) {
	var request Request
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("Invalid request received: %v", err)
	}

	return &request, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request, err := decodeRequest(r.Body)

	if err != nil {
		handler.Logger.Println(err)
		return
	}

	cmd, err := request.makeCmd()

	if err != nil {
		handler.Logger.Println(err)
		return
	}

	id := generateID()
	job := Job{ID: id, Cmd: cmd}

	go func() {
		handler.Jobs <- &job
	}()
}

func (handler *Handler) worker() {
	for job := range handler.Jobs {
		if err := job.Run(); err != nil {
			handler.Logger.Println(err)
		}
	}
}

func main() {
	if _, err := exec.LookPath("lftp"); err != nil {
		log.Fatal("LFTP not found")
	}

	request := Request{
		Path:     "ftp://example.org/path",
		Username: "user",
		Password: "pass",
	}

	logger := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)

	if err := encoder.Encode(request); err != nil {
		log.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		resp, err := http.Post("http://localhost:7800/jsonrpc", "application/json", buffer)

		if err != nil {
			logger.Println(err)
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Println(resp.Status)
		}
	}()

	handler := &Handler{
		Logger: logger,
		Jobs:   make(chan *Job, 10),
	}

	http.Handle("/jsonrpc", handler)
	go handler.worker()
	log.Fatal(http.ListenAndServe(":7800", nil))
}
