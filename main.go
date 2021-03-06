package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"golang.org/x/crypto/bcrypt"
)

var (
	rpcListenPort = flag.Int("rpc-listen-port", 7800, "Specify a port number for JSON-RPC server to listen to. Possible values: 1024-65535")
	rpcSecret     = flag.String("rpc-secret", "", "Set RPC secret authorization token (required)")
	maxRetries    = flag.Int("max-retries", 5, "The maximum number of sequential tries of an operation without success. Possible values: 1-100")

	n = flag.Int("n", 4, "Number of connections to use when downloading single file. Possible values: 1-100")
	o = flag.String("o", "", "Output directory (optional, default value is the current working directory)")
	p = flag.Int("p", 1, "Number of files to download in parallel when mirroring directories. Possible values: 1-10")
	s = flag.String("s", "", "Script to run after successful download")

	connectTimeout = 5 * time.Second

	// Info is used for logging information.
	Info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Error is used for logging errors.
	Error = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	errMissingURL           = errors.New("No URL specified in a request")
	errProtocolMismatch     = errors.New("Only HTTP/FTP downloads are supported")
	errInvalidRequestFormat = errors.New("Invalid request format")
	errTokenMismatch        = errors.New("Secret token does not match")
	errUnauthorized         = errors.New("Missing or invalid credentials")
)

// Request represents single request for mirroring one FTP directory or a file.
type Request struct {
	FileName string `json:"filename"`
	Way      string `json:"way"`
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
	Jobs        chan *Job
	HashedToken []byte
}

// JobID is unique identifier of a job.
type JobID [32]byte

// Job is single download request with associated LFTP command and script that will run after download is completed.
type Job struct {
	ID        *JobID
	Command   *exec.Cmd
	ScriptCmd *exec.Cmd
}

func (request *Request) extractURL() (*url.URL, error) {
	if request.Path == "" {
		return nil, errMissingURL
	}

	url, err := url.Parse(request.Path)

	if err != nil || url.Host == "" {
		return nil, fmt.Errorf("Invalid URL: %s", request.Path)
	}

	return url, nil
}

func makeLftpCmd(url *url.URL, way string, filename string) string {
	escaped := "/"

	if url.Path != "" {
		escaped = strings.Replace(url.Path, "\"", "\\\"", -1)
	}

	var cmd string

	if way == "down" {
		if url.Scheme == "ftp" && strings.HasSuffix(url.Path, "/") {
			cmd = fmt.Sprintf("mirror --parallel=%d --use-pget-n=%d \"%s\" && exit", *p, *n, escaped)
		} else {
			cmd = fmt.Sprintf("pget -n %d \"%s\" && exit", *n, escaped)
		}
	}
	if way == "up" {
		//if url.Scheme == "ftp" && strings.HasSuffix(url.Path, "/") {
		//	cmd = fmt.Sprintf("mirror --parallel=%d --use-pget-n=%d \"%s\" && exit", *p, *n, escaped)
		//} else {
		///put -O /web/IGEHIRDETES/2019/2019-02-03/ SzaszLajos20190203.wav; bye
		cmd = fmt.Sprintf("put -O \"%s\" \"%s\" && exit", escaped, filename)
		//}
	}

	commands := []string{"set cmd:trace true", fmt.Sprintf("set net:max-retries %d", *maxRetries), cmd, "exit"}
	return strings.Join(commands, "; ")
}

func makeCmd(url *url.URL, username, password string, way string, FileName string) *exec.Cmd {
	lftpCmd := makeLftpCmd(url, way, FileName)
	var args []string

	if username != "" && password != "" {
		args = []string{"--user", username, "--password", password, "-e", lftpCmd, url.Host}
	} else {
		args = []string{"-e", lftpCmd, url.Host}
	}

	cmd := exec.Command("lftp", args...)

	cmd.Dir = *o
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func makeScriptCmd(path string, filename string) (*exec.Cmd, error) {
	if *s == "" {
		return nil, nil
	}

	scriptPath, err := filepath.Abs(*s)

	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(*o, filepath.Base(path))
	Info.Println(outputPath)
	//cmd := exec.Command(scriptPath, outputPath)
	//cmd := exec.Command(scriptPath, outputPath, filename)
	cmd := exec.Command(scriptPath, filename)

	cmd.Dir = *o
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func connect(url *url.URL, username, password string) error {
	switch url.Scheme {
	case "http", "https":
		return connectHTTP(url, username, password)
	case "ftp":
		return connectFTP(url, username, password)
	}

	return errProtocolMismatch
}

func connectHTTP(url *url.URL, username, password string) error {
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)

	if err != nil {
		return fmt.Errorf("Unable to connect to %s", url.Host)
	}

	req.SetBasicAuth(username, password)

	client := &http.Client{Timeout: connectTimeout}
	resp, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("Unable to connect to %s", url.Host)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}

	return nil
}

func connectFTP(url *url.URL, username, password string) error {
	host, port, err := net.SplitHostPort(url.Host)

	if err != nil {
		host, port = url.Host, strconv.Itoa(21)
	}

	conn, err := ftp.DialTimeout(net.JoinHostPort(host, port), connectTimeout)

	if err != nil {
		return fmt.Errorf("Unable to connect to %s", url.Host)
	}

	if username != "" && password != "" {
		err = conn.Login(username, password)
	} else {
		err = conn.Login("anonymous", "anonymous")
	}

	if err != nil {
		return errUnauthorized
	}

	conn.Logout()
	return nil
}

func newID() *JobID {
	var id JobID

	if _, err := rand.Read(id[:]); err != nil {
		panic("Random number generator failed")
	}

	return &id
}

func (id *JobID) serialize() string {
	return hex.EncodeToString(id[:])
}

func (id *JobID) String() string {
	return hex.EncodeToString(id[:6])
}

func (handler *Handler) processRequest(r *http.Request) (*JobID, error) {
	id := newID()
	Info.Printf("Received download request %s from %s\n", id, r.RemoteAddr)

	var request Request
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&request); err != nil {
		return nil, errInvalidRequestFormat
	}

	if err := bcrypt.CompareHashAndPassword(handler.HashedToken, []byte(request.Secret)); err != nil {
		return nil, errTokenMismatch
	}

	Info.Printf("Download request %s has URL %s\n", id, request.Path)
	url, err := request.extractURL()

	if err != nil {
		return nil, err
	}

	///Ezt vagy LFTP file-olvasásra is fel kéne okosítani, vagy inkább kihagyni az ellenőrzést...
	//if err = connect(url, request.Username, request.Password); err != nil {
	//	return nil, err
	//}

	cmd := makeCmd(url, request.Username, request.Password, request.Way, request.FileName)
	scriptCmd, err := makeScriptCmd(url.Path, request.FileName)

	if err != nil {
		Error.Printf("Error creating script command for request %s: %s", id, err.Error())
	}

	job := Job{ID: id, Command: cmd, ScriptCmd: scriptCmd}

	go func() {
		handler.Jobs <- &job
	}()

	return id, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := handler.processRequest(r)

	if err == nil {
		json.NewEncoder(w).Encode(Response{ID: id.serialize()})
		return
	}

	Error.Printf("Invalid request received: %s\n", err)
	status := http.StatusBadRequest

	if err == errUnauthorized {
		status = http.StatusUnauthorized
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Message: err.Error()})
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

		if err == nil && job.ScriptCmd != nil {
			//if job.ScriptCmd != nil {
			Info.Printf("Begin script output for request %s", job.ID)
			err = job.ScriptCmd.Run()
			//err := job.ScriptCmd.Run()
			Info.Printf("End script output for request %s", job.ID)

			if err != nil {
				Error.Printf("Failed to execute script for request %s with error: %v\n", job.ID, err)
			} else {
				Info.Printf("Script for request %s completed", job.ID)
			}
		}
	}
}

func getOutputDir(dir string) (string, error) {
	var err error

	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return "", err
		}
	}

	abs, err := filepath.Abs(dir)

	if err != nil {
		return "", err
	}

	file, err := os.Stat(abs)

	if err != nil {
		return "", err
	}

	if !file.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}

	return abs, nil
}

func main() {
	flag.Parse()

	if (*rpcListenPort < 1024 || *rpcListenPort > 65535) || *rpcSecret == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *maxRetries < 1 || *maxRetries > 100 || *n < 1 || *n > 100 || *p < 1 || *p > 10 {
		flag.Usage()
		os.Exit(1)
	}

	if dir, err := getOutputDir(*o); err != nil {
		log.Fatal(err)
	} else {
		*o = dir
	}

	hashedToken, err := bcrypt.GenerateFromPassword([]byte(*rpcSecret), bcrypt.DefaultCost)

	if err != nil {
		log.Fatal("bcrypt failed to generate hashed token")
	}

	//if _, err := exec.LookPath("lftp"); err != nil {
	//	log.Fatal("LFTP not found")
	//}

	handler := &Handler{
		Jobs:        make(chan *Job, 10),
		HashedToken: hashedToken,
	}

	http.Handle("/jsonrpc", handler)
	go handler.worker()

	Info.Printf("Starting LFTP server on port %d\n", *rpcListenPort)
	Info.Printf("Output directory is %s\n", *o)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *rpcListenPort), nil))
}
