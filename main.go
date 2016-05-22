package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var rpcListenPort = flag.Int("rpc-listen-port", 7800, "Specify a port number for JSON-RPC server to listen to. Possible values: 1024-65535")
var rpcSecret = flag.String("rpc-secret", "", "Set RPC secret authorization token (required)")

// Info is used for logging information.
var Info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

// Error is used for logging errors.
var Error = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

// Request represents single request for mirroring one FTP directory or a file.
type Request struct {
	Path     string `json:"path"`
	Username string `json:"username"`
	Password string `json:"password"`
	Secret   string `json:"secret"`
}

// Response represents response to a client with ID for a created job or error message in case of error.
type Response struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// Handler implements http.Handler interface and processes download requests sequentially.
type Handler struct {
	Jobs      chan *Job
	TokenHash []byte
}

// Job is single download request with associated ID and LFTP command.
type Job struct {
	ID      string
	Request *Request
	Command *exec.Cmd
}

func (request *Request) makeCmd() (*exec.Cmd, error) {
	if request.Path == "" {
		return nil, errors.New("No URL specified in a request")
	}

	url, err := url.Parse(request.Path)

	if err != nil {
		return nil, fmt.Errorf("Invalid URL: %s", request.Path)
	}

	if url.Scheme != "ftp" {
		return nil, fmt.Errorf("Only FTP downloads are supported")
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
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		panic("Random number generator failed")
	}

	return base64.StdEncoding.EncodeToString(b)
}

func makeLftpCmd(path string) string {
	if path == "" {
		return "mirror && exit"
	}

	lftpCmd := "pget"

	if strings.HasSuffix(path, "/") {
		lftpCmd = "mirror"
	}

	escaped := strings.Replace(path, "\"", "\\\"", -1)
	return fmt.Sprintf("%s \"%s\" && exit", lftpCmd, escaped)
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
	id := generateID()

	Info.Printf("Received download request %s from %s\n", id, r.RemoteAddr)

	request, err := decodeRequest(r.Body)

	if err != nil {
		serveError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err = bcrypt.CompareHashAndPassword(handler.TokenHash, []byte(request.Secret)); err != nil {
		serveError(w, http.StatusUnauthorized, "Secret token does not match")
		return
	}

	cmd, err := request.makeCmd()

	if err != nil {
		serveError(w, http.StatusBadRequest, err.Error())
		return
	}

	job := Job{ID: id, Request: request, Command: cmd}

	Info.Printf("Download request %s has URL %s\n", id, request.Path)
	json.NewEncoder(w).Encode(Response{ID: id})

	go func() {
		handler.Jobs <- &job
	}()
}

func serveError(w http.ResponseWriter, status int, err string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Message: err})
	Error.Println(err)
}

func (handler *Handler) worker() {
	for job := range handler.Jobs {
		Info.Printf("Begin LFTP output for request %s", job.ID)
		err := job.Command.Run()
		Info.Printf("End LFTP output for request %s", job.ID)

		if err != nil {
			Error.Printf("Failed to execute request %s with error: %v\n", job.ID, err)
		} else {
			Info.Printf("Request %s completed", job.ID)
		}
	}
}

func main() {
	flag.Parse()

	if (*rpcListenPort < 1024 || *rpcListenPort > 65535) || *rpcSecret == "" {
		flag.Usage()
		os.Exit(1)
	}

	tokenHash, err := bcrypt.GenerateFromPassword([]byte(*rpcSecret), bcrypt.DefaultCost)

	if err != nil {
		log.Fatal("bcrypt failed to generate token hash")
	}

	if _, err := exec.LookPath("lftp"); err != nil {
		log.Fatal("LFTP not found")
	}

	handler := &Handler{
		Jobs:      make(chan *Job, 10),
		TokenHash: tokenHash,
	}

	http.Handle("/jsonrpc", handler)
	go handler.worker()

	Info.Printf("Starting LFTP server on port %d\n", *rpcListenPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *rpcListenPort), nil))
}
