package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"code.google.com/p/go-uuid/uuid"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/gorilla/pat"
)

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

type FunctionCall struct {
	Env []models.EnvironmentVariable `json:"env"`
}

type FunctionCallResponse struct {
	Guid string `json:"guid"`
}

func newGuid() string {
	return uuid.NewUUID().String()
}

func registrationHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")
	tarball, _, err := r.FormFile("tarball")
	if err != nil {
		http.Error(w, "no script", http.StatusBadRequest)
		return
	}
	defer tarball.Close()

	path := filepath.Join("functions", name)
	output, err := os.Create(path)
	if err != nil {
		log.Println(err)
		http.Error(w, "could not create function tarball", http.StatusInternalServerError)
		return
	}
	defer output.Close()

	_, err = io.Copy(output, tarball)
	if err != nil {
		log.Println(err)
		http.Error(w, "could not copy function tarball", http.StatusInternalServerError)
	}

	io.WriteString(w, "registered function: "+name)
}

func getFunctionHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")

	http.ServeFile(w, r, "functions/"+name)
}

func callHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")

	path := filepath.Join("functions", name)
	if _, err := os.Stat(path); err != nil {
		http.Error(w, "could not find function", http.StatusNotFound)
		return
	}

	var call FunctionCall
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	downloadAction := &models.DownloadAction{
		From: "http://192.168.59.3:3333/function/" + name,
		To:   "/home/vcap",
	}

	installAction := &models.RunAction{
		Path:       "npm",
		Args:       []string{"install", "/home/vcap/package"},
		Privileged: true,
	}

	executeAction := &models.RunAction{
		Path:       "node_modules/.bin/run",
		Env:        call.Env,
		Privileged: true,
	}

	serialAction := &models.SerialAction{
		Actions: []models.Action{
			downloadAction,
			installAction,
			executeAction,
		},
	}

	guid := newGuid()
	taskCreateRequest := receptor.TaskCreateRequest{
		TaskGuid:              guid,
		LogGuid:               "gamma",
		Domain:                "gamma",
		Stack:                 "lucid64",
		RootFSPath:            "docker:///dockerfile/nodejs",
		Action:                serialAction,
		CompletionCallbackURL: "http://192.168.59.3:3333/callback",
	}

	if err := runTask(taskCreateRequest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := FunctionCallResponse{
		Guid: guid,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		message := fmt.Sprintf("failed to write response: %s", err.Error())
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-type", "application/json")
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(os.Stdout, r.Body)
}

func runTask(request receptor.TaskCreateRequest) error {
	client := receptor.NewClient("http://receptor.192.168.11.11.xip.io")
	return client.CreateTask(request)
}

func main() {
	os.MkdirAll("functions", 0777)
	pat := pat.New()

	pat.Put("/function/{name}", http.HandlerFunc(registrationHandler))
	pat.Get("/function/{name}", http.HandlerFunc(getFunctionHandler))
	pat.Post("/function/{name}/call", http.HandlerFunc(callHandler))
	pat.Post("/callback", http.HandlerFunc(callbackHandler))

	http.Handle("/", pat)
	log.Fatalln(http.ListenAndServe(":3333", nil))
}
