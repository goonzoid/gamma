package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/gorilla/pat"
)

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

var functions map[string]Function = make(map[string]Function)

type Function struct {
	Name   string
	Script string
}

type FunctionCall struct {
	Env []models.EnvironmentVariable `json:"env"`
}

func registrationHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")
	script, _, err := r.FormFile("script")
	if err != nil {
		http.Error(w, "no script", http.StatusBadRequest)
		return
	}

	contents, err := ioutil.ReadAll(script)
	if err != nil {
		http.Error(w, "bad things", http.StatusInternalServerError)
		return
	}
	defer script.Close()

	functions[name] = Function{
		Name:   name,
		Script: string(contents),
	}
}

func callHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get(":name")

	function, found := functions[name]
	if !found {
		http.Error(w, "could not find function", http.StatusNotFound)
		return
	}

	var call FunctionCall
	if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	args := []string{"-e", function.Script}

	action := &models.RunAction{
		Path: "node",
		Args: args,
		Env:  call.Env,
	}

	taskCreateRequest := receptor.TaskCreateRequest{
		TaskGuid:              "pain",
		Domain:                "thatsapar",
		Stack:                 "lucid64",
		RootFSPath:            "docker:///dockerfile/nodejs",
		Action:                action,
		CompletionCallbackURL: "http://192.168.59.3:3333/callback",
		LogGuid:               "tempz",
	}

	if err := runTask(taskCreateRequest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	io.WriteString(w, "ok")
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(os.Stdout, r.Body)
}

func runTask(request receptor.TaskCreateRequest) error {
	client := receptor.NewClient("http://receptor.192.168.11.11.xip.io")
	return client.CreateTask(request)
}

func main() {
	script, err := ioutil.ReadFile("script.js")
	panicIfErr(err)

	functions["default"] = Function{
		Name:   "default",
		Script: string(script),
	}

	pat := pat.New()

	pat.Put("/function/{name}", http.HandlerFunc(registrationHandler))
	pat.Post("/function/{name}/call", http.HandlerFunc(callHandler))
	pat.Post("/callback", http.HandlerFunc(callbackHandler))

	http.Handle("/", pat)
	log.Fatalln(http.ListenAndServe(":3333", nil))
}
